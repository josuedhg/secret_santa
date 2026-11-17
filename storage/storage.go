package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"secret_santa/models"
)

var (
	ErrCodeNotFound        = errors.New("código de invitación no encontrado")
	ErrCodeAlreadyUsed     = errors.New("este código de invitación ya ha sido utilizado")
	ErrRegistrationClosed  = errors.New("el registro está cerrado porque el sorteo ya se realizó")
	ErrNameRequired        = errors.New("el nombre no puede estar vacío")
	ErrNameTaken           = errors.New("este nombre ya ha sido registrado por otro participante")
	ErrParticipantNotFound = errors.New("participante no encontrado")
	ErrCannotDeleteDrawn   = errors.New("no se pueden eliminar participantes después de realizado el sorteo")
)

type Store struct {
	mu       sync.RWMutex
	filePath string
	state    models.AppState
}

// GenerateRandomString generates a secure random string of given length using a URL-safe character set
func GenerateRandomString(n int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Excludes ambiguous chars like I, O, 0, 1
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			result[i] = charset[0]
			continue
		}
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// GenerateID generates a secure hex ID
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewStore initializes or loads the storage
func NewStore(filePath string, defaultAdminKey string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		state: models.AppState{
			Event: models.Event{
				ID:          GenerateID(),
				Title:       "Intercambio de Regalos 🎁",
				Description: "¡Bienvenidos a nuestro intercambio navideño!",
				Budget:      "$25 - $30",
				Status:      models.StatusRegistering,
				CreatedAt:   time.Now(),
				AdminKey:    defaultAdminKey,
			},
			InviteCodes:  make(map[string]models.InviteCode),
			Participants: make(map[string]models.Participant),
		},
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("error al crear el directorio de datos: %w", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if s.state.Event.AdminKey == "" {
			s.state.Event.AdminKey = GenerateRandomString(12)
		}
		if err := s.saveLocked(); err != nil {
			return nil, fmt.Errorf("error al inicializar el almacenamiento: %w", err)
		}
	} else {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error al leer el archivo de datos: %w", err)
		}
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, fmt.Errorf("error al deserializar el estado: %w", err)
		}
		if s.state.InviteCodes == nil {
			s.state.InviteCodes = make(map[string]models.InviteCode)
		}
		if s.state.Participants == nil {
			s.state.Participants = make(map[string]models.Participant)
		}
		if defaultAdminKey != "" && s.state.Event.AdminKey != defaultAdminKey {
			s.state.Event.AdminKey = defaultAdminKey
			_ = s.saveLocked()
		}
	}

	return s, nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("error al formatear estado: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("error al escribir archivo temporal: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return fmt.Errorf("error al confirmar archivo de estado: %w", err)
	}

	return nil
}

// GetEvent returns a copy of the event details
func (s *Store) GetEvent() models.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Event
}

// UpdateEvent updates editable event properties
func (s *Store) UpdateEvent(title, description, budget string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Event.Title = strings.TrimSpace(title)
	s.state.Event.Description = strings.TrimSpace(description)
	s.state.Event.Budget = strings.TrimSpace(budget)

	return s.saveLocked()
}

// GenerateInviteCodes creates n unique invite codes
func (s *Store) GenerateInviteCodes(count int) ([]models.InviteCode, error) {
	if count <= 0 {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	newCodes := make([]models.InviteCode, 0, count)
	for i := 0; i < count; i++ {
		var code string
		for {
			code = "REGALO-" + GenerateRandomString(4) + "-" + GenerateRandomString(4)
			if _, exists := s.state.InviteCodes[code]; !exists {
				break
			}
		}

		inv := models.InviteCode{
			Code:      code,
			CreatedAt: time.Now(),
			Used:      false,
		}
		s.state.InviteCodes[code] = inv
		newCodes = append(newCodes, inv)
	}

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return newCodes, nil
}

// GetInviteCode looks up an invite code
func (s *Store) GetInviteCode(code string) (models.InviteCode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inv, exists := s.state.InviteCodes[strings.ToUpper(strings.TrimSpace(code))]
	return inv, exists
}

// ListInviteCodes returns all invite codes
func (s *Store) ListInviteCodes() []models.InviteCode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.InviteCode, 0, len(s.state.InviteCodes))
	for _, c := range s.state.InviteCodes {
		result = append(result, c)
	}
	return result
}

// RegisterParticipant claims an invite code and registers a new titular participant
func (s *Store) RegisterParticipant(code, name, wishlist string) (*models.Participant, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	cleanName := strings.TrimSpace(name)
	cleanWishlist := strings.TrimSpace(wishlist)

	if cleanName == "" {
		return nil, ErrNameRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Event.Status == models.StatusDrawn {
		return nil, ErrRegistrationClosed
	}

	inv, exists := s.state.InviteCodes[cleanCode]
	if !exists {
		return nil, ErrCodeNotFound
	}
	if inv.Used {
		return nil, ErrCodeAlreadyUsed
	}

	// Regla: No se permiten nombres duplicados (comparación insensible a mayúsculas)
	for _, p := range s.state.Participants {
		if strings.EqualFold(p.Name, cleanName) {
			return nil, fmt.Errorf("%w: '%s'", ErrNameTaken, cleanName)
		}
	}

	pID := GenerateID()
	sessionToken := GenerateID() + GenerateID()

	participant := models.Participant{
		ID:           pID,
		Name:         cleanName,
		Wishlist:     cleanWishlist,
		InviteCode:   cleanCode,
		FamilyID:     pID,
		FamilyName:   cleanName,
		IsHead:       true,
		SessionToken: sessionToken,
		RegisteredAt: time.Now(),
	}

	inv.Used = true
	inv.ParticipantID = pID
	s.state.InviteCodes[cleanCode] = inv
	s.state.Participants[pID] = participant

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return &participant, nil
}

// AddFamilyMember registers an additional participant under the same family group and invite code
func (s *Store) AddFamilyMember(headID, name, wishlist string) (*models.Participant, error) {
	cleanName := strings.TrimSpace(name)
	cleanWishlist := strings.TrimSpace(wishlist)

	if cleanName == "" {
		return nil, ErrNameRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Event.Status == models.StatusDrawn {
		return nil, ErrRegistrationClosed
	}

	head, exists := s.state.Participants[headID]
	if !exists {
		return nil, ErrParticipantNotFound
	}

	// Regla: No se permiten nombres duplicados en el evento
	for _, p := range s.state.Participants {
		if strings.EqualFold(p.Name, cleanName) {
			return nil, fmt.Errorf("%w: '%s'", ErrNameTaken, cleanName)
		}
	}

	mID := GenerateID()
	familyID := head.FamilyID
	if familyID == "" {
		familyID = head.ID
	}
	familyName := head.FamilyName
	if familyName == "" {
		familyName = head.Name
	}

	member := models.Participant{
		ID:           mID,
		Name:         cleanName,
		Wishlist:     cleanWishlist,
		InviteCode:   head.InviteCode,
		FamilyID:     familyID,
		FamilyName:   familyName,
		IsHead:       false,
		SessionToken: head.SessionToken,
		RegisteredAt: time.Now(),
	}

	s.state.Participants[mID] = member

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return &member, nil
}

// GetFamilyMembers returns all participants belonging to a specific family ID
func (s *Store) GetFamilyMembers(familyID string) []models.Participant {
	if familyID == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var members []models.Participant
	for _, p := range s.state.Participants {
		pFamily := p.FamilyID
		if pFamily == "" {
			pFamily = p.ID
		}
		if pFamily == familyID {
			members = append(members, p)
		}
	}

	sort.Slice(members, func(i, j int) bool {
		if members[i].IsHead != members[j].IsHead {
			return members[i].IsHead // Head first
		}
		return members[i].RegisteredAt.Before(members[j].RegisteredAt)
	})

	return members
}

// DeleteFamilyMember removes a non-head family member before the draw
func (s *Store) DeleteFamilyMember(headID, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Event.Status == models.StatusDrawn {
		return ErrCannotDeleteDrawn
	}

	head, exists := s.state.Participants[headID]
	if !exists {
		return ErrParticipantNotFound
	}

	member, exists := s.state.Participants[memberID]
	if !exists {
		return ErrParticipantNotFound
	}

	headFam := head.FamilyID
	if headFam == "" {
		headFam = head.ID
	}
	memberFam := member.FamilyID
	if memberFam == "" {
		memberFam = member.ID
	}

	if headFam != memberFam || member.IsHead {
		return errors.New("no autorizado para eliminar este participante")
	}

	delete(s.state.Participants, memberID)
	return s.saveLocked()
}

// GetParticipant looks up a participant by ID
func (s *Store) GetParticipant(id string) (models.Participant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.state.Participants[id]
	return p, exists
}

// GetParticipantByCode finds the titular participant who redeemed this code
func (s *Store) GetParticipantByCode(code string) (models.Participant, bool) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	s.mu.RLock()
	defer s.mu.RUnlock()

	var fallback models.Participant
	foundFallback := false

	for _, p := range s.state.Participants {
		if p.InviteCode == cleanCode {
			if p.IsHead {
				return p, true
			}
			if !foundFallback {
				fallback = p
				foundFallback = true
			}
		}
	}
	return fallback, foundFallback
}

// GetParticipantBySession finds the primary participant associated with a session cookie
func (s *Store) GetParticipantBySession(sessionToken string) (models.Participant, bool) {
	if sessionToken == "" {
		return models.Participant{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var fallback models.Participant
	foundFallback := false

	for _, p := range s.state.Participants {
		if p.SessionToken == sessionToken {
			if p.IsHead {
				return p, true
			}
			if !foundFallback {
				fallback = p
				foundFallback = true
			}
		}
	}
	return fallback, foundFallback
}

// ListParticipants returns all registered participants
func (s *Store) ListParticipants() []models.Participant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.Participant, 0, len(s.state.Participants))
	for _, p := range s.state.Participants {
		result = append(result, p)
	}
	return result
}

// DeleteParticipant removes a participant before the draw.
// If the participant is the head of a family, it deletes all family members and frees up their invite code.
// If the participant is an extra family member, only that member is removed.
func (s *Store) DeleteParticipant(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Event.Status == models.StatusDrawn {
		return ErrCannotDeleteDrawn
	}

	p, exists := s.state.Participants[id]
	if !exists {
		return ErrParticipantNotFound
	}

	// Si es el titular (o el único con ese código):
	if p.IsHead || p.FamilyID == "" || p.FamilyID == p.ID {
		familyID := p.FamilyID
		if familyID == "" {
			familyID = p.ID
		}

		// Eliminar todos los miembros de este núcleo familiar
		for otherID, otherP := range s.state.Participants {
			otherFam := otherP.FamilyID
			if otherFam == "" {
				otherFam = otherP.ID
			}
			if otherFam == familyID {
				delete(s.state.Participants, otherID)
			}
		}

		// Liberar el código de invitación
		if inv, ok := s.state.InviteCodes[p.InviteCode]; ok {
			inv.Used = false
			inv.ParticipantID = ""
			s.state.InviteCodes[p.InviteCode] = inv
		}
	} else {
		// Solo es un familiar adicional: eliminarlo individualmente
		delete(s.state.Participants, id)
	}

	return s.saveLocked()
}

// SaveDrawResults commits the draw assignments and sets the event status to drawn
func (s *Store) SaveDrawResults(assignments map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Event.Status == models.StatusDrawn {
		return ErrRegistrationClosed
	}

	now := time.Now()
	s.state.Event.Status = models.StatusDrawn
	s.state.Event.DrawnAt = &now

	for giverID, receiverID := range assignments {
		p, exists := s.state.Participants[giverID]
		if exists {
			p.ReceiverID = receiverID
			s.state.Participants[giverID] = p
		}
	}

	return s.saveLocked()
}

// ResetDraw allows resetting the draw back to registering if needed (admin feature)
func (s *Store) ResetDraw() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Event.Status = models.StatusRegistering
	s.state.Event.DrawnAt = nil

	for id, p := range s.state.Participants {
		p.ReceiverID = ""
		s.state.Participants[id] = p
	}

	return s.saveLocked()
}
