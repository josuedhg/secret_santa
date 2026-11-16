package logic

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"secret_santa/models"
)

var (
	ErrNotEnoughParticipants = errors.New("at least 3 participants are recommended, minimum 2 required")
	ErrDuplicateNames        = errors.New("cannot draw with duplicate participant names")
	ErrAlreadyDrawn          = errors.New("event has already been drawn")
)

// CryptoRandInt returns a secure random integer in [0, max)
func CryptoRandInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive, got %d", max)
	}
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(nBig.Int64()), nil
}

// ShuffleCrypto performs an in-place Fisher-Yates shuffle using crypto/rand
func ShuffleCrypto[T any](items []T) error {
	n := len(items)
	for i := n - 1; i > 0; i-- {
		j, err := CryptoRandInt(i + 1)
		if err != nil {
			return err
		}
		items[i], items[j] = items[j], items[i]
	}
	return nil
}

// PerformDraw assigns each participant a unique receiver following Secret Santa rules:
// 1. No one receives their own name (no self-assignments).
// 2. Names cannot be duplicated.
// 3. Every participant gives exactly one gift and receives exactly one gift (bijective / single Hamiltonian cycle).
func PerformDraw(participants []models.Participant) (map[string]string, error) {
	n := len(participants)
	if n < 2 {
		return nil, ErrNotEnoughParticipants
	}

	// Rule 2 verification: check for duplicate names (case-insensitive & trimmed)
	seenNames := make(map[string]bool)
	for _, p := range participants {
		cleanName := strings.ToLower(strings.TrimSpace(p.Name))
		if seenNames[cleanName] {
			return nil, fmt.Errorf("%w: '%s'", ErrDuplicateNames, p.Name)
		}
		seenNames[cleanName] = true
	}

	// Extract participant IDs
	ids := make([]string, n)
	for i, p := range participants {
		ids[i] = p.ID
	}

	// Shuffle participant IDs using crypto/rand
	shuffled := make([]string, n)
	copy(shuffled, ids)
	if err := ShuffleCrypto(shuffled); err != nil {
		return nil, fmt.Errorf("failed to shuffle participants: %w", err)
	}

	// Assign along a single Hamiltonian cycle:
	// shuffled[0] -> shuffled[1] -> ... -> shuffled[n-1] -> shuffled[0]
	// This guarantees:
	// 1. For all i, receiver != giver (since n >= 2 and (i+1)%n != i)
	// 2. Every participant is a giver exactly once
	// 3. Every participant is a receiver exactly once
	// 4. One unified circle, avoiding isolated 2-cycles when n >= 3
	assignments := make(map[string]string, n) // giverID -> receiverID
	for i := 0; i < n; i++ {
		giverID := shuffled[i]
		receiverID := shuffled[(i+1)%n]
		assignments[giverID] = receiverID
	}

	// Sanity checks on the resulting assignments
	receivedCount := make(map[string]int, n)
	for giverID, receiverID := range assignments {
		if giverID == receiverID {
			return nil, fmt.Errorf("self-assignment detected for participant %s", giverID)
		}
		receivedCount[receiverID]++
	}

	for _, id := range ids {
		if receivedCount[id] != 1 {
			return nil, fmt.Errorf("participant %s received %d gifts instead of 1", id, receivedCount[id])
		}
	}

	return assignments, nil
}
