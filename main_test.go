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

func TestEndToEndFlow(t *testing.T) {
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

	// 2. Test GET /
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rec.Code)
	}

	// 3. Test GET /join/{code1}
	req = httptest.NewRequest("GET", "/join/"+code1, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /join/%s, got %d", code1, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), code1) {
		t.Fatalf("expected page to contain code %s", code1)
	}

	// 4. Register Alice with code1
	form := url.Values{}
	form.Set("name", "Alice")
	form.Set("email", "alice@example.com")
	form.Set("wishlist", "Scented candle")

	req = httptest.NewRequest("POST", "/join/"+code1, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 See Other after register, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/reveal/"+code1) {
		t.Fatalf("expected redirect to /reveal/%s, got %s", code1, rec.Header().Get("Location"))
	}

	// 5. Test Rule: Code is single-use - try using code1 again
	form.Set("name", "Mallory")
	form.Set("email", "mallory@example.com")
	req = httptest.NewRequest("POST", "/join/"+code1, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "already+been+used") {
		t.Fatalf("expected code already used redirect error, got %s", rec.Header().Get("Location"))
	}

	// 6. Test Rule: Duplicate names not allowed - try registering "alice" (case-insensitive) with code2
	form.Set("name", "alice") // lower case match
	form.Set("email", "another@example.com")
	req = httptest.NewRequest("POST", "/join/"+code2, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "already+taken") {
		t.Fatalf("expected duplicate name redirect error, got %s", rec.Header().Get("Location"))
	}

	// 7. Register Bob with code2 and Charlie with code3
	form.Set("name", "Bob")
	form.Set("email", "bob@example.com")
	form.Set("wishlist", "Coffee mug")
	req = httptest.NewRequest("POST", "/join/"+code2, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("failed to register Bob: %d", rec.Code)
	}

	form.Set("name", "Charlie")
	form.Set("email", "charlie@example.com")
	form.Set("wishlist", "Dark chocolate")
	req = httptest.NewRequest("POST", "/join/"+code3, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("failed to register Charlie: %d", rec.Code)
	}

	// 8. Check Alice's status before the draw at /reveal/{code1}
	req = httptest.NewRequest("GET", "/reveal/"+code1, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reveal, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Waiting for Draw") {
		t.Fatal("expected Waiting for Draw message before draw")
	}

	// 9. Unauthorized admin roll attempt
	req = httptest.NewRequest("POST", "/admin/roll", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "Unauthorized") {
		t.Fatalf("expected unauthorized redirect for admin roll without key, got %d", rec.Code)
	}

	// 10. Authorized admin roll using cookie
	req = httptest.NewRequest("POST", "/admin/roll", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_auth",
		Value: "test-admin-secret-key",
	})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "success") {
		t.Fatalf("expected success redirect after admin roll, got %d, loc: %s", rec.Code, rec.Header().Get("Location"))
	}

	// 11. Verify Draw Rules in Database:
	// - No self matches
	// - No duplicate recipients
	// - Everyone gets a gift
	participants := store.ListParticipants()
	if len(participants) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(participants))
	}

	givers := make(map[string]string)
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
		givers[p.ID] = p.ReceiverID
	}

	if len(receivers) != 3 {
		t.Fatalf("expected 3 distinct receivers, got %d", len(receivers))
	}

	// 12. Check Alice's status AFTER draw at /reveal/{code1}
	req = httptest.NewRequest("GET", "/reveal/"+code1, nil)
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
