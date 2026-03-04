package conversation

import (
	"context"
	"time"
)

// Repository is the interface for performing CRUD operations on Conversations.
type Repository interface {
	CreateConversation(ctx context.Context, conversation *Conversation) error
	ReadConversation(ctx context.Context, id ConversationID) (*Conversation, error)
	ListConversationsByRepo(ctx context.Context, repoID string) ([]*Conversation, error)
	UpdateConversationStatus(ctx context.Context, id ConversationID, status Status) error
	AppendMessage(ctx context.Context, id ConversationID, msg Message) error
	SetPendingMessage(ctx context.Context, id ConversationID, message *string) error
	SetEpicID(ctx context.Context, id ConversationID, epicID string) error
	DeleteConversation(ctx context.Context, id ConversationID) error

	// Worker support
	ListPendingConversations(ctx context.Context) ([]*Conversation, error)
	ClaimConversation(ctx context.Context, id ConversationID) (bool, error)
	ConversationHeartbeat(ctx context.Context, id ConversationID) error
	ReleaseConversationClaim(ctx context.Context, id ConversationID) error
	ListStaleConversations(ctx context.Context, threshold time.Time) ([]*Conversation, error)

	// ListActiveConversations returns all conversations in active status.
	ListActiveConversations(ctx context.Context) ([]*Conversation, error)
}
