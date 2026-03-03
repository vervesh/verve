package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/sqlite/sqlc"
)

var _ githubtoken.Repository = (*GitHubTokenRepository)(nil)

// GitHubTokenRepository implements githubtoken.Repository using SQLite.
type GitHubTokenRepository struct {
	db *sqlc.Queries
}

// NewGitHubTokenRepository creates a new GitHubTokenRepository backed by the given SQLite DB.
func NewGitHubTokenRepository(dbtx DB) *GitHubTokenRepository {
	return &GitHubTokenRepository{db: sqlc.New(dbtx)}
}

func (r *GitHubTokenRepository) UpsertGitHubToken(ctx context.Context, encryptedToken string, now time.Time) error {
	return r.db.UpsertGitHubToken(ctx, sqlc.UpsertGitHubTokenParams{
		EncryptedToken: encryptedToken,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	})
}

func (r *GitHubTokenRepository) ReadGitHubToken(ctx context.Context) (string, error) {
	token, err := r.db.ReadGitHubToken(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", githubtoken.ErrTokenNotFound
		}
		return "", err
	}
	return token, nil
}

func (r *GitHubTokenRepository) DeleteGitHubToken(ctx context.Context) error {
	return r.db.DeleteGitHubToken(ctx)
}
