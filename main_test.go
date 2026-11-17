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
	tmpDir, err := os.MkdirTemp("", "ss_e2e_*")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Port:     8080,
		BaseURL:  "http://localhost:8080",
		DataPath: filepath.Join(tmpDir, "data.json"),
		AdminKey: "test-admin-secret-key",
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

	// 1. Generate 3 invite codes
	codes, err := store.GenerateInviteCodes(3)
	if err != nil {
		t.Fatalf("failed to generate codes: %v", err)
	}
	code1 := codes[0].Code
	code2 := codes[1].Code
	code3 := codes[2].Code

	// 2. Test GET / when not logged in -> shows welcome form
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rec.Code)
	}

	// 3. Register Alice with code1 (no email, no password)
	form := url.Values{}
	form.Set("name", "Alice")
	form.Set("wishlist", "Scented candle")

	req = httptest.NewRequest("POST", "/join/"+code1, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 See Other after register, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/reveal") {
		t.Fatalf("expected redirect to /reveal, got %s", rec.Header().Get("Location"))
	}

	// Verify session cookie was issued
	cookies := rec.Result().Cookies()
	var aliceCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "santa_participant_session" {
			aliceCookie = c
			break
		}
	}
	if aliceCookie == nil || aliceCookie.Value == "" {
		t.Fatal("expected santa_participant_session cookie to be set")
	}

	// 4. Test visiting / while logged in with aliceCookie -> auto-redirects to /reveal!
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(aliceCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/reveal" {
		t.Fatalf("expected auto-redirect to /reveal for logged in user, got %d to %s", rec.Code, rec.Header().Get("Location"))
	}

	// 5. Test Rule: Code is single-use - try using code1 again
	form.Set("name", "Mallory")
	req = httptest.NewRequest("POST", "/join/"+code1, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "already+been+used") {
		t.Fatalf("expected code already used redirect error, got %s", rec.Header().Get("Location"))
	}

	// 6. Test Rule: Duplicate names not allowed - try registering "alice" (case-insensitive) with code2
	form.Set("name", "alice")
	req = httptest.NewRequest("POST", "/join/"+code2, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "already+taken") {
		t.Fatalf("expected duplicate name redirect error, got %s", rec.Header().Get("Location"))
	}

	// 7. Register Bob with code2 and Charlie with code3
	form.Set("name", "Bob")
	form.Set("wishlist", "Coffee mug")
	req = httptest.NewRequest("POST", "/join/"+code2, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("failed to register Bob: %d", rec.Code)
	}

	form.Set("name", "Charlie")
	form.Set("wishlist", "Dark chocolate")
	req = httptest.NewRequest("POST", "/join/"+code3, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("failed to register Charlie: %d", rec.Code)
	}

	// 8. Check Alice's status before the draw using her session cookie
	req = httptest.NewRequest("GET", "/reveal", nil)
	req.AddCookie(aliceCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reveal, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Draw Pending") {
		t.Fatal("expected Draw Pending message before draw")
	}

	// 9. Admin logs in with key URL parameter /admin?key=test-admin-secret-key
	req = httptest.NewRequest("GET", "/admin?key=test-admin-secret-key", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin dashboard, got %d", rec.Code)
	}

	var adminCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "santa_admin_session" {
			adminCookie = c
			break
		}
	}
	if adminCookie == nil || adminCookie.Value == "" {
		t.Fatal("expected santa_admin_session cookie to be set")
	}

	// 10. Admin rolls Secret Santa
	req = httptest.NewRequest("POST", "/admin/roll", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "success") {
		t.Fatalf("expected success redirect after admin roll, got %d, loc: %s", rec.Code, rec.Header().Get("Location"))
	}

	// 11. Verify Draw Rules in Database:
	// - No self matches
	// - No duplicate recipients
	// - Everyone gives 1 and gets 1 gift
	participants := store.ListParticipants()
	if len(participants) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(participants))
	}

	receivers := make(map[string]bool)
	for _, p := range participants {
		if p.ReceiverID == "" {
			t.Fatalf("participant %s has no receiver assigned", p.Name)
		}
		if p.ReceiverID == p.ID {
			t.Fatalf("RULE VIOLATION: participant %s was assigned themselves!", p.Name)
		}
		if receivers[p.ReceiverID] {
			t.Fatalf("RULE VIOLATION: duplicate receiver %s!", p.ReceiverID)
		}
		receivers[p.ReceiverID] = true
	}

	if len(receivers) != 3 {
		t.Fatalf("expected 3 distinct receivers, got %d", len(receivers))
	}

	// 12. Check Alice's reveal page AFTER draw using her session cookie
	req = httptest.NewRequest("GET", "/reveal", nil)
	req.AddCookie(aliceCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after draw, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Click to Unwrap Your Match") {
		t.Fatal("expected unwrap match interface after draw")
	}

	pAlice, _ := store.GetParticipantByCode(code1)
	receiver, _ := store.GetParticipant(pAlice.ReceiverID)
	if !strings.Contains(body, receiver.Name) {
		t.Fatalf("expected Alice's reveal page to contain receiver name %s", receiver.Name)
	}
}
