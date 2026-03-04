package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/joshjon/kit/errtag"

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

func (r *RepoRepository) UpdateRepoSetupScan(ctx context.Context, id repo.RepoID, result repo.SetupScanResult) error {
	return tagRepoErr(r.db.UpdateRepoSetupScan(ctx, sqlc.UpdateRepoSetupScanParams{
		Summary:     result.Summary,
		TechStack:   marshalJSONStrings(result.TechStack),
		HasCode:     boolToInt64(result.HasCode),
		HasClaudeMd: boolToInt64(result.HasCLAUDEMD),
		HasReadme:   boolToInt64(result.HasREADME),
		SetupStatus: result.SetupStatus,
		ID:          id.String(),
	}))
}

func (r *RepoRepository) UpdateRepoSetupStatus(ctx context.Context, id repo.RepoID, status string) error {
	return tagRepoErr(r.db.UpdateRepoSetupStatus(ctx, sqlc.UpdateRepoSetupStatusParams{
		SetupStatus: status,
		ID:          id.String(),
	}))
}

func (r *RepoRepository) UpdateRepoExpectations(ctx context.Context, id repo.RepoID, update repo.ExpectationsUpdate) error {
	var completedAt *int64
	if update.SetupCompletedAt != nil {
		unix := update.SetupCompletedAt.Unix()
		completedAt = &unix
	}
	return tagRepoErr(r.db.UpdateRepoExpectations(ctx, sqlc.UpdateRepoExpectationsParams{
		Expectations:     update.Expectations,
		SetupCompletedAt: completedAt,
		ID:               id.String(),
	}))
}

func (r *RepoRepository) UpdateRepoSummary(ctx context.Context, id repo.RepoID, summary string) error {
	return tagRepoErr(r.db.UpdateRepoSummary(ctx, sqlc.UpdateRepoSummaryParams{
		Summary: summary,
		ID:      id.String(),
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
		TechStack:        unmarshalJSONStrings(in.TechStack),
		SetupStatus:      in.SetupStatus,
		HasCode:          in.HasCode != 0,
		HasCLAUDEMD:      in.HasClaudeMd != 0,
		HasREADME:        in.HasReadme != 0,
		Expectations:     in.Expectations,
		SetupCompletedAt: unixPtrToTimePtr(in.SetupCompletedAt),
		CreatedAt:        unixToTime(in.CreatedAt),
	}
	return rp
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func tagRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[repo.ErrTagRepoNotFound](err)
	}
	if isSQLiteErrCode(err, sqliteConstraint, sqliteConstraintUnique, sqliteConstraintPrimaryKey) {
		return errtag.Tag[repo.ErrTagRepoConflict](err)
	}
	return err
}
