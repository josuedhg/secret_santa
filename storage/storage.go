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
	"strings"
	"sync"
	"time"

	"secret_santa/models"
)

var (
	ErrCodeNotFound     = errors.New("invitation code not found")
	ErrCodeAlreadyUsed   = errors.New("this invitation code has already been used")
	ErrRegistrationClosed = errors.New("registration is closed because the draw has already occurred")
	ErrNameRequired     = errors.New("name cannot be empty")
	ErrNameTaken        = errors.New("this name is already taken by another participant")
	ErrEmailTaken       = errors.New("this email is already registered")
	ErrParticipantNotFound = errors.New("participant not found")
	ErrCannotDeleteDrawn = errors.New("cannot remove participants after the draw has been completed")
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
			// fallback
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
				Title:       "Secret Santa 🎁",
				Description: "Welcome to our holiday Secret Santa gift exchange!",
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
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if s.state.Event.AdminKey == "" {
			s.state.Event.AdminKey = GenerateRandomString(12)
		}
		if err := s.saveLocked(); err != nil {
			return nil, fmt.Errorf("failed to initialize store: %w", err)
		}
	} else {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read store file: %w", err)
		}
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state: %w", err)
		}
		if s.state.InviteCodes == nil {
			s.state.InviteCodes = make(map[string]models.InviteCode)
		}
		if s.state.Participants == nil {
			s.state.Participants = make(map[string]models.Participant)
		}
		// If an admin key was explicitly passed via env/flag and differs, keep it
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
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write tmp state file: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return fmt.Errorf("failed to commit state file: %w", err)
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
			code = "SANTA-" + GenerateRandomString(4) + "-" + GenerateRandomString(4)
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

// RegisterParticipant claims an invite code and registers a new participant
func (s *Store) RegisterParticipant(code, name, email, wishlist string) (*models.Participant, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	cleanName := strings.TrimSpace(name)
	cleanEmail := strings.TrimSpace(email)
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

	// Check Rule 2: Names cannot be duplicated (case-insensitive check)
	for _, p := range s.state.Participants {
		if strings.EqualFold(p.Name, cleanName) {
			return nil, fmt.Errorf("%w: '%s'", ErrNameTaken, cleanName)
		}
		if cleanEmail != "" && strings.EqualFold(p.Email, cleanEmail) {
			return nil, fmt.Errorf("%w: '%s'", ErrEmailTaken, cleanEmail)
		}
	}

	pID := GenerateID()
	participant := models.Participant{
		ID:           pID,
		Name:         cleanName,
		Email:        cleanEmail,
		Wishlist:     cleanWishlist,
		InviteCode:   cleanCode,
		RegisteredAt: time.Now(),
		EmailSent:    false,
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

// GetParticipant looks up a participant by ID
func (s *Store) GetParticipant(id string) (models.Participant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.state.Participants[id]
	return p, exists
}

// GetParticipantByCode finds the participant who redeemed this code
func (s *Store) GetParticipantByCode(code string) (models.Participant, bool) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.state.Participants {
		if p.InviteCode == cleanCode {
			return p, true
		}
	}
	return models.Participant{}, false
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

// DeleteParticipant removes a participant before the draw and frees up their invite code
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

	// Free the invite code
	if inv, ok := s.state.InviteCodes[p.InviteCode]; ok {
		inv.Used = false
		inv.ParticipantID = ""
		s.state.InviteCodes[p.InviteCode] = inv
	}

	delete(s.state.Participants, id)
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

// UpdateParticipantEmailStatus updates email delivery status
func (s *Store) UpdateParticipantEmailStatus(id string, sent bool, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.state.Participants[id]
	if !exists {
		return ErrParticipantNotFound
	}

	p.EmailSent = sent
	p.EmailError = errMsg
	s.state.Participants[id] = p

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
		p.EmailSent = false
		p.EmailError = ""
		s.state.Participants[id] = p
	}

	return s.saveLocked()
}
