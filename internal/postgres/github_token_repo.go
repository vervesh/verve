package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/postgres/sqlc"
)

var _ githubtoken.Repository = (*GitHubTokenRepository)(nil)

// GitHubTokenRepository implements githubtoken.Repository using PostgreSQL.
type GitHubTokenRepository struct {
	db *sqlc.Queries
}

// NewGitHubTokenRepository creates a new GitHubTokenRepository backed by the given pgx pool.
func NewGitHubTokenRepository(pool *pgxpool.Pool) *GitHubTokenRepository {
	return &GitHubTokenRepository{db: sqlc.New(pool)}
}

func (r *GitHubTokenRepository) UpsertGitHubToken(ctx context.Context, encryptedToken string, now time.Time) error {
	return r.db.UpsertGitHubToken(ctx, sqlc.UpsertGitHubTokenParams{
		EncryptedToken: encryptedToken,
		CreatedAt:      now.Unix(),
	})
}

func (r *GitHubTokenRepository) ReadGitHubToken(ctx context.Context) (string, error) {
	token, err := r.db.ReadGitHubToken(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", githubtoken.ErrTokenNotFound
		}
		return "", err
	}
	return token, nil
}

func (r *GitHubTokenRepository) DeleteGitHubToken(ctx context.Context) error {
	return r.db.DeleteGitHubToken(ctx)
}
