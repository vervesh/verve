package conversation

import "time"

// Status represents the lifecycle state of a Conversation.
type Status string

const (
	StatusActive   Status = "active"   // Conversation is ongoing, accepting messages
	StatusArchived Status = "archived" // Conversation has been archived
)

// Message represents a single message in a conversation.
type Message struct {
	Role      string `json:"role"`      // "user" or "assistant"
	Content   string `json:"content"`   // Message content
	Timestamp int64  `json:"timestamp"` // Unix epoch
}

// Conversation represents a chat conversation scoped to a repository.
type Conversation struct {
	ID              ConversationID `json:"id"`
	RepoID          string         `json:"repo_id"`
	Title           string         `json:"title"`
	Status          Status         `json:"status"`
	Messages        []Message      `json:"messages"`
	Model           string         `json:"model,omitempty"`
	ClaimedAt       *time.Time     `json:"claimed_at,omitempty"`
	LastHeartbeatAt *time.Time     `json:"last_heartbeat_at,omitempty"`
	PendingMessage  *string        `json:"pending_message,omitempty"`
	EpicID          *string        `json:"epic_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// NewConversation creates a new Conversation in active status.
func NewConversation(repoID, title, model string) *Conversation {
	now := time.Now()
	return &Conversation{
		ID:        NewConversationID(),
		RepoID:    repoID,
		Title:     title,
		Status:    StatusActive,
		Messages:  []Message{},
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
