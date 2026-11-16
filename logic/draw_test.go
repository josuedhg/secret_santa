package logic

import (
	"fmt"
	"testing"

	"secret_santa/models"
)

func TestPerformDraw_Validation(t *testing.T) {
	// < 2 participants
	_, err := PerformDraw([]models.Participant{
		{ID: "1", Name: "Alice"},
	})
	if err == nil {
		t.Fatal("expected error for 1 participant, got nil")
	}

	// Duplicate names (case-insensitive)
	_, err = PerformDraw([]models.Participant{
		{ID: "1", Name: "Alice"},
		{ID: "2", Name: "Bob"},
		{ID: "3", Name: "alice"},
	})
	if err == nil {
		t.Fatal("expected error for duplicate names, got nil")
	}
}

func TestPerformDraw_RulesEnforced(t *testing.T) {
	testSizes := []int{2, 3, 4, 5, 10, 50}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
			participants := make([]models.Participant, size)
			for i := 0; i < size; i++ {
				participants[i] = models.Participant{
					ID:   fmt.Sprintf("user-%d", i),
					Name: fmt.Sprintf("Person %d", i),
				}
			}

			// Run 100 iterations per size to verify randomness and rule adherence
			for iter := 0; iter < 100; iter++ {
				assignments, err := PerformDraw(participants)
				if err != nil {
					t.Fatalf("unexpected error at iter %d: %v", iter, err)
				}

				if len(assignments) != size {
					t.Fatalf("expected %d assignments, got %d", size, len(assignments))
				}

				receivedCounts := make(map[string]int)
				for giverID, receiverID := range assignments {
					// Rule 1: No one can get their own name
					if giverID == receiverID {
						t.Fatalf("rule violation: participant %s got themselves!", giverID)
					}
					receivedCounts[receiverID]++
				}

				// Rule 3: Every participant receives exactly 1 gift
				for _, p := range participants {
					count := receivedCounts[p.ID]
					if count != 1 {
						t.Fatalf("rule violation: participant %s received %d gifts (expected 1)", p.ID, count)
					}
				}
			}
		})
	}
}
