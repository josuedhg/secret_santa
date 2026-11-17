package logic

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"secret_santa/models"
)

var (
	ErrNotEnoughParticipants = errors.New("se requieren al menos 2 participantes para realizar el intercambio (recomendado 3+)")
	ErrDuplicateNames        = errors.New("no se puede realizar el sorteo con nombres duplicados")
	ErrAlreadyDrawn          = errors.New("el intercambio ya ha sido sorteado")
	ErrFamilyTooLarge        = errors.New("un grupo familiar tiene demasiados integrantes para realizar el sorteo")
	ErrNoValidAssignment     = errors.New("no fue posible encontrar una asignación que respete todas las restricciones familiares")
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

// effectiveFamilyID returns the family ID or the participant ID if no family ID is set
func effectiveFamilyID(p models.Participant) string {
	if strings.TrimSpace(p.FamilyID) != "" {
		return strings.TrimSpace(p.FamilyID)
	}
	return p.ID
}

// PerformDraw assigns each participant a unique receiver following the Intercambio rules:
// 1. Nadie puede autoregalarse (no self-assignments).
// 2. Los nombres no pueden estar duplicados.
// 3. Ningún participante puede regalarle a un integrante de su propia familia.
// 4. Cada persona da exactamente un regalo y recibe exactamente un regalo (biyección perfecta).
func PerformDraw(participants []models.Participant) (map[string]string, error) {
	n := len(participants)
	if n < 2 {
		return nil, ErrNotEnoughParticipants
	}

	// Regla 2: Comprobar nombres duplicados (insensible a mayúsculas y espacios en blanco)
	seenNames := make(map[string]bool)
	for _, p := range participants {
		cleanName := strings.ToLower(strings.TrimSpace(p.Name))
		if cleanName == "" {
			return nil, errors.New("hay participantes con el nombre vacío")
		}
		if seenNames[cleanName] {
			return nil, fmt.Errorf("%w: '%s'", ErrDuplicateNames, p.Name)
		}
		seenNames[cleanName] = true
	}

	// Contar integrantes por familia y verificar condición de posibilidad (Pigeonhole principle)
	familyCounts := make(map[string]int)
	familyNameMap := make(map[string]string)
	for _, p := range participants {
		fID := effectiveFamilyID(p)
		familyCounts[fID]++
		if p.FamilyName != "" {
			familyNameMap[fID] = p.FamilyName
		} else if familyNameMap[fID] == "" {
			familyNameMap[fID] = p.Name
		}
	}

	for fID, count := range familyCounts {
		if count*2 > n {
			fName := familyNameMap[fID]
			return nil, fmt.Errorf("%w: la familia '%s' tiene %d de %d participantes. Ninguna familia puede superar la mitad del total de participantes para evitar regalos entre familiares", ErrFamilyTooLarge, fName, count, n)
		}
	}

	// Intentar resolver la asignación con backtracking aleatorizado
	// Realizamos varios intentos si fuese necesario por aleatoriedad
	const maxAttempts = 50
	for attempt := 0; attempt < maxAttempts; attempt++ {
		assignments := solveDraw(participants, familyCounts)
		if assignments != nil {
			// Verificación de integridad rigurosa
			if err := verifyDraw(participants, assignments); err == nil {
				return assignments, nil
			}
		}
	}

	return nil, ErrNoValidAssignment
}

func solveDraw(participants []models.Participant, familyCounts map[string]int) map[string]string {
	n := len(participants)
	givers := make([]models.Participant, n)
	copy(givers, participants)

	// Heurística MRV: ordenar givers por el tamaño de su familia descendente
	// (los que tienen familias más grandes tienen menos receptores posibles y deben asignarse primero)
	sort.SliceStable(givers, func(i, j int) bool {
		fI := effectiveFamilyID(givers[i])
		fJ := effectiveFamilyID(givers[j])
		return familyCounts[fI] > familyCounts[fJ]
	})

	assignedReceivers := make(map[string]bool, n)
	result := make(map[string]string, n)

	if backtrack(0, givers, participants, assignedReceivers, result) {
		return result
	}

	return nil
}

func backtrack(index int, givers []models.Participant, allParticipants []models.Participant, assignedReceivers map[string]bool, result map[string]string) bool {
	if index == len(givers) {
		return true
	}

	giver := givers[index]
	giverFamily := effectiveFamilyID(giver)

	// Filtrar receptores candidatos válidos para este giver
	var candidates []models.Participant
	for _, candidate := range allParticipants {
		if assignedReceivers[candidate.ID] {
			continue
		}
		if candidate.ID == giver.ID {
			continue // No autoregalo
		}
		if effectiveFamilyID(candidate) == giverFamily {
			continue // No regalo entre familiares
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return false
	}

	// Barajar candidatos aleatoriamente para garantizar equidad y dinamismo
	_ = ShuffleCrypto(candidates)

	for _, cand := range candidates {
		assignedReceivers[cand.ID] = true
		result[giver.ID] = cand.ID

		if backtrack(index+1, givers, allParticipants, assignedReceivers, result) {
			return true
		}

		delete(result, giver.ID)
		delete(assignedReceivers, cand.ID)
	}

	return false
}

func verifyDraw(participants []models.Participant, assignments map[string]string) error {
	n := len(participants)
	if len(assignments) != n {
		return fmt.Errorf("número de asignaciones inválido: %d de %d", len(assignments), n)
	}

	pMap := make(map[string]models.Participant, n)
	for _, p := range participants {
		pMap[p.ID] = p
	}

	receivedCounts := make(map[string]int, n)
	for giverID, receiverID := range assignments {
		giver, okG := pMap[giverID]
		receiver, okR := pMap[receiverID]
		if !okG || !okR {
			return fmt.Errorf("participante desconocido en asignación: %s -> %s", giverID, receiverID)
		}

		// Regla 1: No autoregalo
		if giverID == receiverID {
			return fmt.Errorf("autoasignación detectada para %s", giver.Name)
		}

		// Regla 3: No regalos entre integrantes de la misma familia
		if effectiveFamilyID(giver) == effectiveFamilyID(receiver) {
			return fmt.Errorf("violación de regla familiar: %s y %s pertenecen al mismo grupo familiar", giver.Name, receiver.Name)
		}

		receivedCounts[receiverID]++
	}

	// Regla 4: Cada persona recibe exactamente un regalo
	for _, p := range participants {
		if receivedCounts[p.ID] != 1 {
			return fmt.Errorf("participante %s recibió %d regalos (se esperaba 1)", p.Name, receivedCounts[p.ID])
		}
	}

	return nil
}
