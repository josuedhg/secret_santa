package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreFlow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "intercambio_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.json")
	store, err := NewStore(dbPath, "test-admin-key")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Generate invite codes
	codes, err := store.GenerateInviteCodes(3)
	if err != nil {
		t.Fatalf("failed to generate codes: %v", err)
	}
	if len(codes) != 3 {
		t.Fatalf("expected 3 codes, got %d", len(codes))
	}

	code1 := codes[0].Code
	code2 := codes[1].Code
	code3 := codes[2].Code

	// 2. Register participant with code1
	p1, err := store.RegisterParticipant(code1, "Alicia Gómez", "Libros")
	if err != nil {
		t.Fatalf("failed to register p1: %v", err)
	}
	if p1.Name != "Alicia Gómez" {
		t.Fatalf("expected Alicia Gómez, got %s", p1.Name)
	}
	if p1.SessionToken == "" {
		t.Fatal("expected non-empty SessionToken")
	}

	// Verify session lookup
	bySession, ok := store.GetParticipantBySession(p1.SessionToken)
	if !ok || bySession.ID != p1.ID {
		t.Fatalf("failed to get participant by session token")
	}

	// 3. Try registering again with the same code (must fail)
	_, err = store.RegisterParticipant(code1, "Roberto", "")
	if err != ErrCodeAlreadyUsed {
		t.Fatalf("expected ErrCodeAlreadyUsed, got: %v", err)
	}

	// 4. Try registering with a duplicate name (case-insensitive check)
	_, err = store.RegisterParticipant(code2, "alicia gómez", "")
	if err == nil || !strings.Contains(err.Error(), "ya ha sido registrado") {
		t.Fatalf("expected duplicate name error, got: %v", err)
	}

	// 5. Register Bob and Charlie
	p2, err := store.RegisterParticipant(code2, "Roberto Martínez", "Chocolates")
	if err != nil {
		t.Fatalf("failed to register p2: %v", err)
	}

	p3, err := store.RegisterParticipant(code3, "Carlos Sánchez", "Café de grano")
	if err != nil {
		t.Fatalf("failed to register p3: %v", err)
	}

	// 6. Look up participant by code
	found, ok := store.GetParticipantByCode(code1)
	if !ok || found.ID != p1.ID {
		t.Fatalf("failed to find participant by code %s", code1)
	}

	// 7. Verify all participants listed
	participants := store.ListParticipants()
	if len(participants) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(participants))
	}

	// 8. Save draw results
	assignments := map[string]string{
		p1.ID: p2.ID,
		p2.ID: p3.ID,
		p3.ID: p1.ID,
	}
	if err := store.SaveDrawResults(assignments); err != nil {
		t.Fatalf("failed to save draw results: %v", err)
	}

	// 9. Verify persistence across store re-instantiation
	store2, err := NewStore(dbPath, "test-admin-key")
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}

	reloadedP1, ok := store2.GetParticipant(p1.ID)
	if !ok {
		t.Fatal("p1 not found after reload")
	}
	if reloadedP1.ReceiverID != p2.ID {
		t.Fatalf("expected receiver %s, got %s", p2.ID, reloadedP1.ReceiverID)
	}
}
