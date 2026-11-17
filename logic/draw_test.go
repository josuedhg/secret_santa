package logic

import (
	"fmt"
	"testing"

	"secret_santa/models"
)

func TestPerformDraw_Validation(t *testing.T) {
	// Menos de 2 participantes
	_, err := PerformDraw([]models.Participant{
		{ID: "1", Name: "Alicia"},
	})
	if err == nil {
		t.Fatal("se esperaba error para 1 participante, pero fue nil")
	}

	// Nombres duplicados (insensible a mayúsculas)
	_, err = PerformDraw([]models.Participant{
		{ID: "1", Name: "Alicia"},
		{ID: "2", Name: "Roberto"},
		{ID: "3", Name: "alicia"},
	})
	if err == nil {
		t.Fatal("se esperaba error para nombres duplicados, pero fue nil")
	}
}

func TestPerformDraw_RulesEnforced(t *testing.T) {
	testSizes := []int{2, 3, 4, 5, 10, 50}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("Tamaño_%d", size), func(t *testing.T) {
			participants := make([]models.Participant, size)
			for i := 0; i < size; i++ {
				participants[i] = models.Participant{
					ID:   fmt.Sprintf("usuario-%d", i),
					Name: fmt.Sprintf("Persona %d", i),
				}
			}

			// Probar 100 iteraciones por tamaño
			for iter := 0; iter < 100; iter++ {
				assignments, err := PerformDraw(participants)
				if err != nil {
					t.Fatalf("error inesperado en iteración %d: %v", iter, err)
				}

				if len(assignments) != size {
					t.Fatalf("se esperaban %d asignaciones, pero se obtuvieron %d", size, len(assignments))
				}

				receivedCounts := make(map[string]int)
				for giverID, receiverID := range assignments {
					// Regla 1: Nadie puede sacar su propio nombre
					if giverID == receiverID {
						t.Fatalf("violación de regla: ¡el participante %s se autoregaló!", giverID)
					}
					receivedCounts[receiverID]++
				}

				// Regla 3: Todos reciben exactamente 1 regalo
				for _, p := range participants {
					count := receivedCounts[p.ID]
					if count != 1 {
						t.Fatalf("violación de regla: el participante %s recibió %d regalos (se esperaba 1)", p.ID, count)
					}
				}
			}
		})
	}
}
