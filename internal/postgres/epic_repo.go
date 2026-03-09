package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joshjon/kit/errtag"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/postgres/sqlc"
)

var _ epic.Repository = (*EpicRepository)(nil)

// EpicRepository implements epic.Repository using PostgreSQL.
type EpicRepository struct {
	db *sqlc.Queries
}

// NewEpicRepository creates a new EpicRepository backed by the given pgx pool.
func NewEpicRepository(pool *pgxpool.Pool) *EpicRepository {
	return &EpicRepository{
		db: sqlc.New(pool),
	}
}

func (r *EpicRepository) CreateEpic(ctx context.Context, e *epic.Epic) error {
	proposedJSON, _ := json.Marshal(e.ProposedTasks)
	var prompt *string
	if e.PlanningPrompt != "" {
		prompt = &e.PlanningPrompt
	}
	var model *string
	if e.Model != "" {
		model = &e.Model
	}
	err := r.db.CreateEpic(ctx, sqlc.CreateEpicParams{
		ID:             e.ID.String(),
		RepoID:         e.RepoID,
		Title:          e.Title,
		Description:    e.Description,
		Status:         string(e.Status),
		ProposedTasks:  proposedJSON,
		TaskIds:        e.TaskIDs,
		PlanningPrompt: prompt,
		SessionLog:     e.SessionLog,
		NotReady:       e.NotReady,
		Model:          model,
		CreatedAt:      e.CreatedAt.Unix(),
		UpdatedAt:      e.UpdatedAt.Unix(),
	})
	if err != nil {
		return tagEpicErr(err)
	}

	num, err := r.db.AssignEpicNumber(ctx, sqlc.AssignEpicNumberParams{
		RepoID: e.RepoID,
		ID:     e.ID.String(),
	})
	if err != nil {
		return tagEpicErr(err)
	}
	if num != nil {
		e.Number = int(*num)
	}

	return nil
}

func (r *EpicRepository) ReadEpic(ctx context.Context, id epic.EpicID) (*epic.Epic, error) {
	row, err := r.db.ReadEpic(ctx, id.String())
	if err != nil {
		return nil, tagEpicErr(err)
	}
	return unmarshalEpic(row), nil
}

func (r *EpicRepository) ReadEpicByNumber(ctx context.Context, repoID string, number int) (*epic.Epic, error) {
	num := safeInt32(number)
	row, err := r.db.ReadEpicByNumber(ctx, sqlc.ReadEpicByNumberParams{
		RepoID: repoID,
		Number: &num,
	})
	if err != nil {
		return nil, tagEpicErr(err)
	}
	return unmarshalEpic(row), nil
}

func (r *EpicRepository) ListEpics(ctx context.Context) ([]*epic.Epic, error) {
	rows, err := r.db.ListEpics(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) ListEpicsByRepo(ctx context.Context, repoID string) ([]*epic.Epic, error) {
	rows, err := r.db.ListEpicsByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) UpdateEpic(ctx context.Context, e *epic.Epic) error {
	proposedJSON, _ := json.Marshal(e.ProposedTasks)
	var prompt *string
	if e.PlanningPrompt != "" {
		prompt = &e.PlanningPrompt
	}
	var model *string
	if e.Model != "" {
		model = &e.Model
	}
	return tagEpicErr(r.db.UpdateEpic(ctx, sqlc.UpdateEpicParams{
		ID:             e.ID.String(),
		Title:          e.Title,
		Description:    e.Description,
		Status:         string(e.Status),
		ProposedTasks:  proposedJSON,
		TaskIds:        e.TaskIDs,
		PlanningPrompt: prompt,
		SessionLog:     e.SessionLog,
		NotReady:       e.NotReady,
		Model:          model,
	}))
}

func (r *EpicRepository) UpdateEpicStatus(ctx context.Context, id epic.EpicID, status epic.Status) error {
	return tagEpicErr(r.db.UpdateEpicStatus(ctx, sqlc.UpdateEpicStatusParams{
		ID:     id.String(),
		Status: string(status),
	}))
}

func (r *EpicRepository) UpdateProposedTasks(ctx context.Context, id epic.EpicID, tasks []epic.ProposedTask) error {
	proposedJSON, _ := json.Marshal(tasks)
	return tagEpicErr(r.db.UpdateProposedTasks(ctx, sqlc.UpdateProposedTasksParams{
		ID:            id.String(),
		ProposedTasks: proposedJSON,
	}))
}

func (r *EpicRepository) SetTaskIDs(ctx context.Context, id epic.EpicID, taskIDs []string) error {
	return tagEpicErr(r.db.SetEpicTaskIDs(ctx, sqlc.SetEpicTaskIDsParams{
		ID:      id.String(),
		TaskIds: taskIDs,
	}))
}

func (r *EpicRepository) AppendSessionLog(ctx context.Context, id epic.EpicID, lines []string) error {
	return tagEpicErr(r.db.AppendSessionLog(ctx, sqlc.AppendSessionLogParams{
		ID:         id.String(),
		SessionLog: lines,
	}))
}

func (r *EpicRepository) DeleteEpic(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.DeleteEpic(ctx, id.String()))
}

func (r *EpicRepository) ListPlanningEpics(ctx context.Context) ([]*epic.Epic, error) {
	rows, err := r.db.ListPlanningEpics(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) ClaimEpic(ctx context.Context, id epic.EpicID) (bool, error) {
	rows, err := r.db.ClaimEpic(ctx, id.String())
	return rows > 0, tagEpicErr(err)
}

func (r *EpicRepository) EpicHeartbeat(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.EpicHeartbeat(ctx, id.String()))
}

func (r *EpicRepository) SetEpicFeedback(ctx context.Context, id epic.EpicID, feedback, feedbackType string) error {
	return tagEpicErr(r.db.SetEpicFeedback(ctx, sqlc.SetEpicFeedbackParams{
		ID:           id.String(),
		Feedback:     &feedback,
		FeedbackType: &feedbackType,
	}))
}

func (r *EpicRepository) ClearEpicFeedback(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ClearEpicFeedback(ctx, id.String()))
}

func (r *EpicRepository) ReleaseEpicClaim(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ReleaseEpicClaim(ctx, id.String()))
}

func (r *EpicRepository) ListStaleEpics(ctx context.Context, threshold time.Time) ([]*epic.Epic, error) {
	rows, err := r.db.ListStaleEpics(ctx, ptr(threshold.Unix()))
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) ListActiveEpics(ctx context.Context) ([]*epic.Epic, error) {
	rows, err := r.db.ListActiveEpics(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) RemoveTaskID(ctx context.Context, id epic.EpicID, taskID string) error {
	return tagEpicErr(r.db.RemoveEpicTaskID(ctx, sqlc.RemoveEpicTaskIDParams{
		ID:          id.String(),
		ArrayRemove: taskID,
	}))
}

func unmarshalEpic(in *sqlc.Epic) *epic.Epic {
	e := &epic.Epic{
		ID:           epic.MustParseEpicID(in.ID),
		RepoID:       in.RepoID,
		Title:        in.Title,
		Description:  in.Description,
		Status:       epic.Status(in.Status),
		TaskIDs:      in.TaskIds,
		SessionLog:   in.SessionLog,
		NotReady:     in.NotReady,
		Feedback:     in.Feedback,
		FeedbackType: in.FeedbackType,
	}
	if in.Number != nil {
		e.Number = int(*in.Number)
	}
	if in.PlanningPrompt != nil {
		e.PlanningPrompt = *in.PlanningPrompt
	}
	if in.Model != nil {
		e.Model = *in.Model
	}
	_ = json.Unmarshal(in.ProposedTasks, &e.ProposedTasks)
	if e.ProposedTasks == nil {
		e.ProposedTasks = []epic.ProposedTask{}
	}
	if e.TaskIDs == nil {
		e.TaskIDs = []string{}
	}
	if e.SessionLog == nil {
		e.SessionLog = []string{}
	}
	e.CreatedAt = unixToTime(in.CreatedAt)
	e.UpdatedAt = unixToTime(in.UpdatedAt)
	e.ClaimedAt = unixPtrToTimePtr(in.ClaimedAt)
	e.LastHeartbeatAt = unixPtrToTimePtr(in.LastHeartbeatAt)
	return e
}

func unmarshalEpicList(in []*sqlc.Epic) []*epic.Epic {
	out := make([]*epic.Epic, len(in))
	for i := range in {
		out[i] = unmarshalEpic(in[i])
	}
	return out
}

func tagEpicErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errtag.Tag[epic.ErrTagEpicNotFound](err)
	}
	if isPGErrCode(err, pgerrcode.UniqueViolation) {
		return errtag.Tag[epic.ErrTagEpicConflict](err)
	}
	return err
}
