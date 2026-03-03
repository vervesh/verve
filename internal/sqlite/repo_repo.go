package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/joshjon/kit/errtag"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite/sqlc"
)

var _ repo.Repository = (*RepoRepository)(nil)

// RepoRepository implements repo.Repository using SQLite.
type RepoRepository struct {
	db *sqlc.Queries
}

// NewRepoRepository creates a new RepoRepository backed by the given SQLite DB.
func NewRepoRepository(db DB) *RepoRepository {
	return &RepoRepository{db: sqlc.New(db)}
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
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[repo.ErrTagRepoNotFound](err)
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		if code == sqlite3.SQLITE_CONSTRAINT || code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
			return errtag.Tag[repo.ErrTagRepoConflict](err)
		}
	}
	return err
}
