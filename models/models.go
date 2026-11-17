package models

import (
	"time"
)

// EventStatus defines the state of the Secret Santa event
type EventStatus string

const (
	StatusRegistering EventStatus = "registering"
	StatusDrawn       EventStatus = "drawn"
)

// InviteCode represents a single-use invitation code
type InviteCode struct {
	Code          string    `json:"code"`
	CreatedAt     time.Time `json:"created_at"`
	Used          bool      `json:"used"`
	ParticipantID string    `json:"participant_id,omitempty"`
}

// Participant represents a registered person in the Secret Santa
type Participant struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Wishlist     string    `json:"wishlist,omitempty"`
	InviteCode   string    `json:"invite_code"`
	SessionToken string    `json:"session_token"` // Used to keep the participant logged in via session cookie
	RegisteredAt time.Time `json:"registered_at"`
	ReceiverID   string    `json:"receiver_id,omitempty"` // ID of the person they need to give a gift to
}

// Event holds the overall Secret Santa event configuration and state
type Event struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Budget      string      `json:"budget,omitempty"`
	Status      EventStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	DrawnAt     *time.Time  `json:"drawn_at,omitempty"`
	AdminKey    string      `json:"admin_key"`
}

// AppState represents the complete persisted database
type AppState struct {
	Event        Event                  `json:"event"`
	InviteCodes  map[string]InviteCode  `json:"invite_codes"`
	Participants map[string]Participant `json:"participants"`
}
