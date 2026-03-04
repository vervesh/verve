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

func (r *RepoRepository) UpdateRepoSetupScan(ctx context.Context, id repo.RepoID, result repo.SetupScanResult) error {
	return tagRepoErr(r.db.UpdateRepoSetupScan(ctx, sqlc.UpdateRepoSetupScanParams{
		ID:          id.String(),
		Summary:     result.Summary,
		TechStack:   result.TechStack,
		HasCode:     result.HasCode,
		HasClaudeMd: result.HasCLAUDEMD,
		HasReadme:   result.HasREADME,
		SetupStatus: result.SetupStatus,
	}))
}

func (r *RepoRepository) UpdateRepoSetupStatus(ctx context.Context, id repo.RepoID, status string) error {
	return tagRepoErr(r.db.UpdateRepoSetupStatus(ctx, sqlc.UpdateRepoSetupStatusParams{
		ID:          id.String(),
		SetupStatus: status,
	}))
}

func (r *RepoRepository) UpdateRepoExpectations(ctx context.Context, id repo.RepoID, update repo.ExpectationsUpdate) error {
	var completedAt *int64
	if update.SetupCompletedAt != nil {
		unix := update.SetupCompletedAt.Unix()
		completedAt = &unix
	}
	return tagRepoErr(r.db.UpdateRepoExpectations(ctx, sqlc.UpdateRepoExpectationsParams{
		ID:               id.String(),
		Expectations:     update.Expectations,
		SetupCompletedAt: completedAt,
	}))
}

func (r *RepoRepository) UpdateRepoSummary(ctx context.Context, id repo.RepoID, summary string) error {
	return tagRepoErr(r.db.UpdateRepoSummary(ctx, sqlc.UpdateRepoSummaryParams{
		ID:      id.String(),
		Summary: summary,
	}))
}

func (r *RepoRepository) UpdateRepoTechStack(ctx context.Context, id repo.RepoID, techStack []string) error {
	return tagRepoErr(r.db.UpdateRepoTechStack(ctx, sqlc.UpdateRepoTechStackParams{
		ID:        id.String(),
		TechStack: techStack,
	}))
}

func (r *RepoRepository) ListReposBySetupStatus(ctx context.Context, status string) ([]*repo.Repo, error) {
	rows, err := r.db.ListReposBySetupStatus(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make([]*repo.Repo, len(rows))
	for i, row := range rows {
		out[i] = unmarshalRepo(row)
	}
	return out, nil
}

func unmarshalRepo(in *sqlc.Repo) *repo.Repo {
	rp := &repo.Repo{
		ID:               repo.MustParseRepoID(in.ID),
		Owner:            in.Owner,
		Name:             in.Name,
		FullName:         in.FullName,
		Summary:          in.Summary,
		TechStack:        in.TechStack,
		SetupStatus:      in.SetupStatus,
		HasCode:          in.HasCode,
		HasCLAUDEMD:      in.HasClaudeMd,
		HasREADME:        in.HasReadme,
		Expectations:     in.Expectations,
		SetupCompletedAt: unixPtrToTimePtr(in.SetupCompletedAt),
		CreatedAt:        unixToTime(in.CreatedAt),
	}
	if rp.TechStack == nil {
		rp.TechStack = []string{}
	}
	return rp
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
