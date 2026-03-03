package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joshjon/kit/errtag"

	"github.com/joshjon/verve/internal/postgres/sqlc"
	"github.com/joshjon/verve/internal/repo"
)

var _ repo.Repository = (*RepoRepository)(nil)

// RepoRepository implements repo.Repository using PostgreSQL.
type RepoRepository struct {
	db *sqlc.Queries
}

// NewRepoRepository creates a new RepoRepository backed by the given pgx pool.
func NewRepoRepository(pool *pgxpool.Pool) *RepoRepository {
	return &RepoRepository{db: sqlc.New(pool)}
}

func (r *RepoRepository) CreateRepo(ctx context.Context, rp *repo.Repo) error {
	return tagRepoErr(r.db.CreateRepo(ctx, sqlc.CreateRepoParams{
		ID:        rp.ID.String(),
		Owner:     rp.Owner,
		Name:      rp.Name,
		FullName:  rp.FullName,
		CreatedAt: rp.CreatedAt.Unix(),
	}))
}

func (r *RepoRepository) ReadRepo(ctx context.Context, id repo.RepoID) (*repo.Repo, error) {
	row, err := r.db.ReadRepo(ctx, id.String())
	if err != nil {
		return nil, tagRepoErr(err)
	}
	return unmarshalRepo(row), nil
}

func (r *RepoRepository) ReadRepoByFullName(ctx context.Context, fullName string) (*repo.Repo, error) {
	row, err := r.db.ReadRepoByFullName(ctx, fullName)
	if err != nil {
		return nil, tagRepoErr(err)
	}
	return unmarshalRepo(row), nil
}

func (r *RepoRepository) ListRepos(ctx context.Context) ([]*repo.Repo, error) {
	rows, err := r.db.ListRepos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*repo.Repo, len(rows))
	for i, row := range rows {
		out[i] = unmarshalRepo(row)
	}
	return out, nil
}

func (r *RepoRepository) DeleteRepo(ctx context.Context, id repo.RepoID) error {
	return tagRepoErr(r.db.DeleteRepo(ctx, id.String()))
}

func unmarshalRepo(in *sqlc.Repo) *repo.Repo {
	return &repo.Repo{
		ID:        repo.MustParseRepoID(in.ID),
		Owner:     in.Owner,
		Name:      in.Name,
		FullName:  in.FullName,
		CreatedAt: unixToTime(in.CreatedAt),
	}
}

func tagRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errtag.Tag[repo.ErrTagRepoNotFound](err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return errtag.Tag[repo.ErrTagRepoConflict](err)
	}
	return err
}
