package conversation

import (
	"context"
	"sync"
	"time"

	"github.com/joshjon/kit/log"
)

// Store wraps a Repository and adds application-level concerns for conversations.
type Store struct {
	repo   Repository
	logger log.Logger

	// Pending conversation notification (same pattern as task.Store and epic.Store)
	pendingMu sync.Mutex
	pendingCh chan struct{}
}

// NewStore creates a new Store backed by the given Repository.
func NewStore(repo Repository, logger log.Logger) *Store {
	return &Store{
		repo:      repo,
		logger:    logger.With("component", "conversation_store"),
		pendingCh: make(chan struct{}, 1),
	}
}

// WaitForPending returns a channel that signals when a pending conversation might be available.
func (s *Store) WaitForPending() <-chan struct{} {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return s.pendingCh
}

func (s *Store) notifyPending() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	select {
	case s.pendingCh <- struct{}{}:
	default:
	}
}

// CreateConversation creates a new conversation.
func (s *Store) CreateConversation(ctx context.Context, conv *Conversation) error {
	return s.repo.CreateConversation(ctx, conv)
}

// ReadConversation reads a conversation by ID.
func (s *Store) ReadConversation(ctx context.Context, id ConversationID) (*Conversation, error) {
	return s.repo.ReadConversation(ctx, id)
}

// ListConversationsByRepo returns all conversations for a given repo.
func (s *Store) ListConversationsByRepo(ctx context.Context, repoID string) ([]*Conversation, error) {
	return s.repo.ListConversationsByRepo(ctx, repoID)
}

// DeleteConversation deletes a conversation.
func (s *Store) DeleteConversation(ctx context.Context, id ConversationID) error {
	return s.repo.DeleteConversation(ctx, id)
}

// SendMessage appends a user message, sets pending_message, and notifies pending.
func (s *Store) SendMessage(ctx context.Context, id ConversationID, content string) error {
	msg := Message{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now().Unix(),
	}
	if err := s.repo.AppendMessage(ctx, id, msg); err != nil {
		return err
	}
	if err := s.repo.SetPendingMessage(ctx, id, &content); err != nil {
		return err
	}
	s.notifyPending()
	return nil
}

// ClaimPendingConversation finds an unclaimed pending conversation and claims it atomically.
func (s *Store) ClaimPendingConversation(ctx context.Context) (*Conversation, error) {
	convos, err := s.repo.ListPendingConversations(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range convos {
		ok, err := s.repo.ClaimConversation(ctx, c.ID)
		if err != nil {
			continue
		}
		if !ok {
			continue // Already claimed by another worker
		}
		// Re-read to get updated claimed_at
		return s.repo.ReadConversation(ctx, c.ID)
	}
	return nil, nil
}

// CompleteResponse appends an assistant message, clears pending_message, and releases the claim.
func (s *Store) CompleteResponse(ctx context.Context, id ConversationID, response string) error {
	msg := Message{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now().Unix(),
	}
	if err := s.repo.AppendMessage(ctx, id, msg); err != nil {
		return err
	}
	if err := s.repo.SetPendingMessage(ctx, id, nil); err != nil {
		return err
	}
	return s.repo.ReleaseConversationClaim(ctx, id)
}

// FailResponse clears pending_message and releases the claim on failure.
func (s *Store) FailResponse(ctx context.Context, id ConversationID) error {
	if err := s.repo.SetPendingMessage(ctx, id, nil); err != nil {
		return err
	}
	return s.repo.ReleaseConversationClaim(ctx, id)
}

// ConversationHeartbeat updates the heartbeat timestamp for a claimed conversation.
func (s *Store) ConversationHeartbeat(ctx context.Context, id ConversationID) error {
	return s.repo.ConversationHeartbeat(ctx, id)
}

// SetEpicID links a conversation to a generated epic.
func (s *Store) SetEpicID(ctx context.Context, id ConversationID, epicID string) error {
	return s.repo.SetEpicID(ctx, id, epicID)
}

// TimeoutStaleConversations releases claimed conversations whose heartbeat has expired.
func (s *Store) TimeoutStaleConversations(ctx context.Context, timeout time.Duration) (int, error) {
	threshold := time.Now().Add(-timeout)
	convos, err := s.repo.ListStaleConversations(ctx, threshold)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, c := range convos {
		if err := s.repo.ReleaseConversationClaim(ctx, c.ID); err != nil {
			continue
		}
		count++
		s.notifyPending()
	}
	return count, nil
}

// ArchiveOldConversations archives active conversations whose updated_at is older
// than the retention duration.
func (s *Store) ArchiveOldConversations(ctx context.Context, retention time.Duration) (int, error) {
	threshold := time.Now().Add(-retention)
	convos, err := s.repo.ListActiveConversations(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, c := range convos {
		if c.UpdatedAt.Before(threshold) {
			if err := s.repo.UpdateConversationStatus(ctx, c.ID, StatusArchived); err != nil {
				continue
			}
			count++
		}
	}
	return count, nil
}
