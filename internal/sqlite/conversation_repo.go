package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/joshjon/kit/errtag"

	"github.com/joshjon/verve/internal/conversation"
	"github.com/joshjon/verve/internal/sqlite/sqlc"
)

var _ conversation.Repository = (*ConversationRepository)(nil)

// ConversationRepository implements conversation.Repository using SQLite.
type ConversationRepository struct {
	db *sqlc.Queries
}

// NewConversationRepository creates a new ConversationRepository backed by the given SQLite DB.
func NewConversationRepository(db DB) *ConversationRepository {
	return &ConversationRepository{
		db: sqlc.New(db),
	}
}

func (r *ConversationRepository) CreateConversation(ctx context.Context, c *conversation.Conversation) error {
	messagesJSON, _ := json.Marshal(c.Messages)
	var model *string
	if c.Model != "" {
		model = &c.Model
	}
	err := r.db.CreateConversation(ctx, sqlc.CreateConversationParams{
		ID:             c.ID.String(),
		RepoID:         c.RepoID,
		Title:          c.Title,
		Status:         string(c.Status),
		Messages:       string(messagesJSON),
		Model:          model,
		PendingMessage: c.PendingMessage,
		EpicID:         c.EpicID,
		CreatedAt:      c.CreatedAt.Unix(),
		UpdatedAt:      c.UpdatedAt.Unix(),
	})
	return tagConversationErr(err)
}

func (r *ConversationRepository) ReadConversation(ctx context.Context, id conversation.ConversationID) (*conversation.Conversation, error) {
	row, err := r.db.ReadConversation(ctx, id.String())
	if err != nil {
		return nil, tagConversationErr(err)
	}
	return unmarshalConversation(row), nil
}

func (r *ConversationRepository) ListConversationsByRepo(ctx context.Context, repoID string) ([]*conversation.Conversation, error) {
	rows, err := r.db.ListConversationsByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return unmarshalConversationList(rows), nil
}

func (r *ConversationRepository) UpdateConversationStatus(ctx context.Context, id conversation.ConversationID, status conversation.Status) error {
	return tagConversationErr(r.db.UpdateConversationStatus(ctx, sqlc.UpdateConversationStatusParams{
		Status: string(status),
		ID:     id.String(),
	}))
}

func (r *ConversationRepository) AppendMessage(ctx context.Context, id conversation.ConversationID, msg conversation.Message) error {
	// SQLite doesn't support JSONB concat, so read-modify-write
	existing, err := r.db.ReadConversation(ctx, id.String())
	if err != nil {
		return tagConversationErr(err)
	}
	var messages []conversation.Message
	_ = json.Unmarshal([]byte(existing.Messages), &messages)
	messages = append(messages, msg)
	messagesJSON, _ := json.Marshal(messages)
	return tagConversationErr(r.db.SetConversationMessages(ctx, sqlc.SetConversationMessagesParams{
		Messages: string(messagesJSON),
		ID:       id.String(),
	}))
}

func (r *ConversationRepository) SetPendingMessage(ctx context.Context, id conversation.ConversationID, message *string) error {
	return tagConversationErr(r.db.SetPendingMessage(ctx, sqlc.SetPendingMessageParams{
		PendingMessage: message,
		ID:             id.String(),
	}))
}

func (r *ConversationRepository) SetEpicID(ctx context.Context, id conversation.ConversationID, epicID string) error {
	return tagConversationErr(r.db.SetConversationEpicID(ctx, sqlc.SetConversationEpicIDParams{
		EpicID: &epicID,
		ID:     id.String(),
	}))
}

func (r *ConversationRepository) DeleteConversation(ctx context.Context, id conversation.ConversationID) error {
	return tagConversationErr(r.db.DeleteConversation(ctx, id.String()))
}

func (r *ConversationRepository) ListPendingConversations(ctx context.Context) ([]*conversation.Conversation, error) {
	rows, err := r.db.ListPendingConversations(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalConversationList(rows), nil
}

func (r *ConversationRepository) ClaimConversation(ctx context.Context, id conversation.ConversationID) (bool, error) {
	rows, err := r.db.ClaimConversation(ctx, id.String())
	return rows > 0, tagConversationErr(err)
}

func (r *ConversationRepository) ConversationHeartbeat(ctx context.Context, id conversation.ConversationID) error {
	return tagConversationErr(r.db.ConversationHeartbeat(ctx, id.String()))
}

func (r *ConversationRepository) ReleaseConversationClaim(ctx context.Context, id conversation.ConversationID) error {
	return tagConversationErr(r.db.ReleaseConversationClaim(ctx, id.String()))
}

func (r *ConversationRepository) ListStaleConversations(ctx context.Context, threshold time.Time) ([]*conversation.Conversation, error) {
	thresholdUnix := threshold.Unix()
	rows, err := r.db.ListStaleConversations(ctx, &thresholdUnix)
	if err != nil {
		return nil, err
	}
	return unmarshalConversationList(rows), nil
}

func (r *ConversationRepository) ListActiveConversations(ctx context.Context) ([]*conversation.Conversation, error) {
	rows, err := r.db.ListActiveConversations(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalConversationList(rows), nil
}

func unmarshalConversation(in *sqlc.Conversation) *conversation.Conversation {
	c := &conversation.Conversation{
		ID:              conversation.MustParseConversationID(in.ID),
		RepoID:          in.RepoID,
		Title:           in.Title,
		Status:          conversation.Status(in.Status),
		PendingMessage:  in.PendingMessage,
		EpicID:          in.EpicID,
		ClaimedAt:       unixPtrToTimePtr(in.ClaimedAt),
		LastHeartbeatAt: unixPtrToTimePtr(in.LastHeartbeatAt),
		CreatedAt:       unixToTime(in.CreatedAt),
		UpdatedAt:       unixToTime(in.UpdatedAt),
	}
	if in.Model != nil {
		c.Model = *in.Model
	}
	_ = json.Unmarshal([]byte(in.Messages), &c.Messages)
	if c.Messages == nil {
		c.Messages = []conversation.Message{}
	}
	return c
}

func unmarshalConversationList(in []*sqlc.Conversation) []*conversation.Conversation {
	out := make([]*conversation.Conversation, len(in))
	for i := range in {
		out[i] = unmarshalConversation(in[i])
	}
	return out
}

func tagConversationErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[conversation.ErrTagConversationNotFound](err)
	}
	if isSQLiteErrCode(err, sqliteConstraint, sqliteConstraintUnique, sqliteConstraintPrimaryKey) {
		return errtag.Tag[conversation.ErrTagConversationConflict](err)
	}
	return err
}
