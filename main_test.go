package main

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"secret_santa/config"
	"secret_santa/handlers"
	"secret_santa/storage"
)

//go:embed templates/*
var testTemplateFS embed.FS

//go:embed static/*
var testStaticFS embed.FS

func setupTestServer(t *testing.T) (*handlers.Server, *storage.Store, *http.ServeMux, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "intercambio_e2e_*")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Port:     8080,
		BaseURL:  "http://localhost:8080",
		DataPath: filepath.Join(tmpDir, "data.json"),
		AdminKey: "clave-secreta-organizador",
	}

	store, err := storage.NewStore(cfg.DataPath, cfg.AdminKey)
	if err != nil {
		t.Fatal(err)
	}

	staticSub, err := fs.Sub(testStaticFS, "static")
	if err != nil {
		t.Fatal(err)
	}

	server, err := handlers.NewServer(cfg, store, testTemplateFS, http.FS(staticSub))
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	return server, store, mux, tmpDir
}

func TestEndToEndSessionFlow(t *testing.T) {
	_, store, mux, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// 1. Generar 3 códigos de invitación
	codes, err := store.GenerateInviteCodes(3)
	if err != nil {
		t.Fatalf("error al generar códigos: %v", err)
	}
	code1 := codes[0].Code
	code2 := codes[1].Code
	code3 := codes[2].Code

	// 2. GET / muestra formulario de inicio
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 OK para /, obtenido %d", rec.Code)
	}

	// 3. Registrar a Alicia con code1
	form := url.Values{}
	form.Set("name", "Alicia")
	form.Set("wishlist", "Vela aromática")

	req = httptest.NewRequest("POST", "/join/"+code1, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("se esperaba 303 See Other después de registrar, obtenido %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/reveal") {
		t.Fatalf("se esperaba redirección a /reveal, obtenido %s", rec.Header().Get("Location"))
	}

	// Verificar cookie de sesión del participante
	cookies := rec.Result().Cookies()
	var aliciaCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "intercambio_session" {
			aliciaCookie = c
			break
		}
	}
	if aliciaCookie == nil || aliciaCookie.Value == "" {
		t.Fatal("se esperaba cookie intercambio_session establecida")
	}

	// 4. Probar entrar a / teniendo sesión activa -> auto-redirecciona a /reveal!
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(aliciaCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/reveal" {
		t.Fatalf("se esperaba auto-redirección a /reveal para usuario conectado, obtenido %d hacia %s", rec.Code, rec.Header().Get("Location"))
	}

	// 5. Regla: Código de un solo uso - intentar usar code1 otra vez
	form.Set("name", "Mónica")
	req = httptest.NewRequest("POST", "/join/"+code1, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "utilizado") {
		t.Fatalf("se esperaba error de código ya utilizado, obtenido %s", rec.Header().Get("Location"))
	}

	// 6. Regla: Nombres duplicados no permitidos - intentar registrar "alicia" con code2
	form.Set("name", "alicia")
	req = httptest.NewRequest("POST", "/join/"+code2, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "registrado") {
		t.Fatalf("se esperaba error de nombre duplicado, obtenido %s", rec.Header().Get("Location"))
	}

	// 7. Registrar a Roberto con code2 y Carlos con code3
	form.Set("name", "Roberto")
	form.Set("wishlist", "Taza para café")
	req = httptest.NewRequest("POST", "/join/"+code2, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("falló registro de Roberto: %d", rec.Code)
	}

	form.Set("name", "Carlos")
	form.Set("wishlist", "Chocolate amargo")
	req = httptest.NewRequest("POST", "/join/"+code3, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("falló registro de Carlos: %d", rec.Code)
	}

	// 8. Revisar estado de Alicia antes del sorteo
	req = httptest.NewRequest("GET", "/reveal", nil)
	req.AddCookie(aliciaCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 OK para reveal, obtenido %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Sorteo pendiente") {
		t.Fatal("se esperaba mensaje de Sorteo pendiente antes de sortear")
	}

	// 9. Organizador accede con clave URL param /admin?key=clave-secreta-organizador
	req = httptest.NewRequest("GET", "/admin?key=clave-secreta-organizador", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 OK para panel de organizador, obtenido %d", rec.Code)
	}

	var adminCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "intercambio_admin_session" {
			adminCookie = c
			break
		}
	}
	if adminCookie == nil || adminCookie.Value == "" {
		t.Fatal("se esperaba cookie intercambio_admin_session establecida")
	}

	// 10. Organizador realiza el sorteo
	req = httptest.NewRequest("POST", "/admin/roll", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "success") {
		t.Fatalf("se esperaba redirección con éxito tras sorteo, obtenido %d, loc: %s", rec.Code, rec.Header().Get("Location"))
	}

	// 11. Verificar reglas del intercambio en base de datos:
	// - Sin autoregalos
	// - Sin receptores duplicados
	// - Todos dan 1 y reciben 1 regalo
	participants := store.ListParticipants()
	if len(participants) != 3 {
		t.Fatalf("se esperaban 3 participantes, obtenido %d", len(participants))
	}

	receivers := make(map[string]bool)
	for _, p := range participants {
		if p.ReceiverID == "" {
			t.Fatalf("participante %s no tiene receptor asignado", p.Name)
		}
		if p.ReceiverID == p.ID {
			t.Fatalf("VIOLACIÓN DE REGLA: ¡el participante %s se autoregaló!", p.Name)
		}
		if receivers[p.ReceiverID] {
			t.Fatalf("VIOLACIÓN DE REGLA: ¡receptor duplicado %s!", p.ReceiverID)
		}
		receivers[p.ReceiverID] = true
	}

	if len(receivers) != 3 {
		t.Fatalf("se esperaban 3 receptores distintos, obtenido %d", len(receivers))
	}

	// 12. Revisar página de Alicia DESPUÉS del sorteo usando su cookie de sesión
	req = httptest.NewRequest("GET", "/reveal", nil)
	req.AddCookie(aliciaCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 OK tras sorteo, obtenido %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "¡Toca para descubrir a quién le regalas!") {
		t.Fatal("se esperaba tarjeta interactiva para revelar match tras el sorteo")
	}

	pAlicia, _ := store.GetParticipantByCode(code1)
	receiver, _ := store.GetParticipant(pAlicia.ReceiverID)
	if !strings.Contains(body, receiver.Name) {
		t.Fatalf("se esperaba que la página de Alicia contuviera el nombre de su destinatario %s", receiver.Name)
	}
}
