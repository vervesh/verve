package tome

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Record stores a new session in the database.
func (t *Tome) Record(ctx context.Context, s Session) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	if s.Status == "" {
		s.Status = "succeeded"
	}
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Files == nil {
		s.Files = []string{}
	}

	tags, err := json.Marshal(s.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	files, err := json.Marshal(s.Files)
	if err != nil {
		return fmt.Errorf("marshal files: %w", err)
	}

	_, err = t.db.ExecContext(ctx, `
		INSERT INTO session (id, summary, learnings, content, tags, files, branch, status, transcript_hash, user, repo, created_at, exported)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
	`, s.ID, s.Summary, s.Learnings, s.Content, string(tags), string(files), s.Branch, s.Status, nullString(s.TranscriptHash), s.User, s.Repo, s.CreatedAt.Unix())
	return err
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
