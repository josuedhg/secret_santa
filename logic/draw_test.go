package logic

import (
	"errors"
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

func TestPerformDraw_FamilyTooLarge(t *testing.T) {
	// 3 participantes, donde 2 pertenecen a la misma familia (2 > 3/2 -> 2*2 > 3)
	participants := []models.Participant{
		{ID: "1", Name: "Papá García", FamilyID: "fam-garcia", FamilyName: "García"},
		{ID: "2", Name: "Hijo García", FamilyID: "fam-garcia", FamilyName: "García"},
		{ID: "3", Name: "Carlos Solo", FamilyID: "fam-carlos"},
	}

	_, err := PerformDraw(participants)
	if err == nil {
		t.Fatal("se esperaba error de familia demasiado grande, pero no hubo error")
	}
	if !errors.Is(err, ErrFamilyTooLarge) {
		t.Fatalf("se esperaba ErrFamilyTooLarge, se obtuvo: %v", err)
	}
}

func TestPerformDraw_NoFamilyViolations(t *testing.T) {
	// 2 familias de 2 personas cada una (Total: 4)
	participants := []models.Participant{
		{ID: "1", Name: "Papá Gómez", FamilyID: "fam-gomez", FamilyName: "Familia Gómez"},
		{ID: "2", Name: "Hijo Gómez", FamilyID: "fam-gomez", FamilyName: "Familia Gómez"},
		{ID: "3", Name: "Mamá López", FamilyID: "fam-lopez", FamilyName: "Familia López"},
		{ID: "4", Name: "Hija López", FamilyID: "fam-lopez", FamilyName: "Familia López"},
	}

	for iter := 0; iter < 100; iter++ {
		assignments, err := PerformDraw(participants)
		if err != nil {
			t.Fatalf("error inesperado en iteración %d: %v", iter, err)
		}

		if len(assignments) != 4 {
			t.Fatalf("se esperaban 4 asignaciones, se obtuvieron %d", len(assignments))
		}

		pMap := make(map[string]models.Participant)
		for _, p := range participants {
			pMap[p.ID] = p
		}

		receivedCount := make(map[string]int)
		for giverID, receiverID := range assignments {
			giver := pMap[giverID]
			receiver := pMap[receiverID]

			if giverID == receiverID {
				t.Fatalf("iter %d: autoasignación para %s", iter, giver.Name)
			}
			if giver.FamilyID == receiver.FamilyID {
				t.Fatalf("iter %d: regalo intrafamiliar prohibido entre %s y %s", iter, giver.Name, receiver.Name)
			}
			receivedCount[receiverID]++
		}

		for _, p := range participants {
			if receivedCount[p.ID] != 1 {
				t.Fatalf("iter %d: participante %s recibió %d regalos", iter, p.Name, receivedCount[p.ID])
			}
		}
	}
}

func TestPerformDraw_MultipleFamiliesAndSingles(t *testing.T) {
	// Varias familias y participantes individuales
	participants := []models.Participant{
		{ID: "1", Name: "Papá Pérez", FamilyID: "fam-1", FamilyName: "Pérez"},
		{ID: "2", Name: "Mamá Pérez", FamilyID: "fam-1", FamilyName: "Pérez"},
		{ID: "3", Name: "Hijo Pérez", FamilyID: "fam-1", FamilyName: "Pérez"},

		{ID: "4", Name: "Tío Gómez", FamilyID: "fam-2", FamilyName: "Gómez"},
		{ID: "5", Name: "Tía Gómez", FamilyID: "fam-2", FamilyName: "Gómez"},

		{ID: "6", Name: "Primo Juan", FamilyID: "fam-3"},
		{ID: "7", Name: "Amiga Ana", FamilyID: "fam-4"},
		{ID: "8", Name: "Vecino Pedro", FamilyID: "fam-5"},
	}

	for iter := 0; iter < 100; iter++ {
		assignments, err := PerformDraw(participants)
		if err != nil {
			t.Fatalf("error en iteración %d: %v", iter, err)
		}

		pMap := make(map[string]models.Participant)
		for _, p := range participants {
			pMap[p.ID] = p
		}

		for giverID, receiverID := range assignments {
			giver := pMap[giverID]
			receiver := pMap[receiverID]

			if giverID == receiverID {
				t.Fatalf("autoasignación para %s", giver.Name)
			}
			if effectiveFamilyID(giver) == effectiveFamilyID(receiver) {
				t.Fatalf("violación familiar: %s y %s comparten la misma familia (%s)", giver.Name, receiver.Name, giver.FamilyID)
			}
		}
	}
}

func TestPerformDraw_IndividualParticipantsLegacy(t *testing.T) {
	testSizes := []int{2, 3, 5, 10, 30}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("Tamaño_%d", size), func(t *testing.T) {
			participants := make([]models.Participant, size)
			for i := 0; i < size; i++ {
				participants[i] = models.Participant{
					ID:   fmt.Sprintf("usuario-%d", i),
					Name: fmt.Sprintf("Persona %d", i),
				}
			}

			for iter := 0; iter < 50; iter++ {
				assignments, err := PerformDraw(participants)
				if err != nil {
					t.Fatalf("error en iter %d: %v", iter, err)
				}
				if len(assignments) != size {
					t.Fatalf("se esperaban %d asignaciones, se obtuvieron %d", size, len(assignments))
				}
			}
		})
	}
}
