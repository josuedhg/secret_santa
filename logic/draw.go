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
	ErrNotEnoughParticipants = errors.New("se requieren al menos 2 participantes para realizar el intercambio (recomendado 3+)")
	ErrDuplicateNames        = errors.New("no se puede realizar el sorteo con nombres duplicados")
	ErrAlreadyDrawn          = errors.New("el intercambio ya ha sido sorteado")
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

// PerformDraw assigns each participant a unique receiver following the Intercambio rules:
// 1. Nadie puede autoregalarse (no self-assignments).
// 2. Los nombres no pueden estar duplicados.
// 3. Cada persona da exactamente un regalo y recibe exactamente un regalo (ciclo hamiltoniano único).
func PerformDraw(participants []models.Participant) (map[string]string, error) {
	n := len(participants)
	if n < 2 {
		return nil, ErrNotEnoughParticipants
	}

	// Regla 2: Comprobar nombres duplicados (insensible a mayúsculas y espacios en blanco)
	seenNames := make(map[string]bool)
	for _, p := range participants {
		cleanName := strings.ToLower(strings.TrimSpace(p.Name))
		if seenNames[cleanName] {
			return nil, fmt.Errorf("%w: '%s'", ErrDuplicateNames, p.Name)
		}
		seenNames[cleanName] = true
	}

	// Extraer IDs de los participantes
	ids := make([]string, n)
	for i, p := range participants {
		ids[i] = p.ID
	}

	// Barajar con crypto/rand
	shuffled := make([]string, n)
	copy(shuffled, ids)
	if err := ShuffleCrypto(shuffled); err != nil {
		return nil, fmt.Errorf("error al barajar participantes: %w", err)
	}

	// Asignación mediante ciclo hamiltoniano:
	// shuffled[0] -> shuffled[1] -> ... -> shuffled[n-1] -> shuffled[0]
	// Esto garantiza:
	// 1. Para todo i, receptor != emisor (ya que n >= 2 y (i+1)%n != i)
	// 2. Cada participante da exactamente un regalo
	// 3. Cada participante recibe exactamente un regalo
	// 4. Se evitan intercambios cerrados de 2 personas cuando n >= 3
	assignments := make(map[string]string, n)
	for i := 0; i < n; i++ {
		giverID := shuffled[i]
		receiverID := shuffled[(i+1)%n]
		assignments[giverID] = receiverID
	}

	// Verificación de integridad
	receivedCount := make(map[string]int, n)
	for giverID, receiverID := range assignments {
		if giverID == receiverID {
			return nil, fmt.Errorf("se detectó autoasignación para el participante %s", giverID)
		}
		receivedCount[receiverID]++
	}

	for _, id := range ids {
		if receivedCount[id] != 1 {
			return nil, fmt.Errorf("el participante %s recibió %d regalos en lugar de 1", id, receivedCount[id])
		}
	}

	return assignments, nil
}
