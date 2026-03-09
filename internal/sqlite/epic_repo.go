package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/joshjon/kit/errtag"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/sqlite/sqlc"
)

var _ epic.Repository = (*EpicRepository)(nil)

// EpicRepository implements epic.Repository using SQLite.
type EpicRepository struct {
	db *sqlc.Queries
}

// NewEpicRepository creates a new EpicRepository backed by the given SQLite DB.
func NewEpicRepository(db DB) *EpicRepository {
	return &EpicRepository{
		db: sqlc.New(db),
	}
}

func (r *EpicRepository) CreateEpic(ctx context.Context, e *epic.Epic) error {
	proposedJSON, _ := json.Marshal(e.ProposedTasks)
	taskIDsJSON, _ := json.Marshal(e.TaskIDs)
	sessionLogJSON, _ := json.Marshal(e.SessionLog)
	var prompt *string
	if e.PlanningPrompt != "" {
		prompt = &e.PlanningPrompt
	}
	var notReady int64
	if e.NotReady {
		notReady = 1
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
		ProposedTasks:  string(proposedJSON),
		TaskIds:        string(taskIDsJSON),
		PlanningPrompt: prompt,
		SessionLog:     string(sessionLogJSON),
		NotReady:       notReady,
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
	num := int64(number)
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
	taskIDsJSON, _ := json.Marshal(e.TaskIDs)
	sessionLogJSON, _ := json.Marshal(e.SessionLog)
	var prompt *string
	if e.PlanningPrompt != "" {
		prompt = &e.PlanningPrompt
	}
	var notReady int64
	if e.NotReady {
		notReady = 1
	}
	var model *string
	if e.Model != "" {
		model = &e.Model
	}
	return tagEpicErr(r.db.UpdateEpic(ctx, sqlc.UpdateEpicParams{
		Title:          e.Title,
		Description:    e.Description,
		Status:         string(e.Status),
		ProposedTasks:  string(proposedJSON),
		TaskIds:        string(taskIDsJSON),
		PlanningPrompt: prompt,
		SessionLog:     string(sessionLogJSON),
		NotReady:       notReady,
		Model:          model,
		ID:             e.ID.String(),
	}))
}

func (r *EpicRepository) UpdateEpicStatus(ctx context.Context, id epic.EpicID, status epic.Status) error {
	return tagEpicErr(r.db.UpdateEpicStatus(ctx, sqlc.UpdateEpicStatusParams{
		Status: string(status),
		ID:     id.String(),
	}))
}

func (r *EpicRepository) UpdateProposedTasks(ctx context.Context, id epic.EpicID, tasks []epic.ProposedTask) error {
	proposedJSON, _ := json.Marshal(tasks)
	return tagEpicErr(r.db.UpdateProposedTasks(ctx, sqlc.UpdateProposedTasksParams{
		ProposedTasks: string(proposedJSON),
		ID:            id.String(),
	}))
}

func (r *EpicRepository) SetTaskIDs(ctx context.Context, id epic.EpicID, taskIDs []string) error {
	taskIDsJSON, _ := json.Marshal(taskIDs)
	return tagEpicErr(r.db.SetEpicTaskIDs(ctx, sqlc.SetEpicTaskIDsParams{
		TaskIds: string(taskIDsJSON),
		ID:      id.String(),
	}))
}

func (r *EpicRepository) AppendSessionLog(ctx context.Context, id epic.EpicID, lines []string) error {
	// SQLite doesn't support array_append, so read-modify-write
	existing, err := r.db.ReadEpic(ctx, id.String())
	if err != nil {
		return tagEpicErr(err)
	}
	var existingLog []string
	_ = json.Unmarshal([]byte(existing.SessionLog), &existingLog)
	existingLog = append(existingLog, lines...)
	logJSON, _ := json.Marshal(existingLog)
	return tagEpicErr(r.db.AppendSessionLog(ctx, sqlc.AppendSessionLogParams{
		SessionLog: string(logJSON),
		ID:         id.String(),
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
		Feedback:     &feedback,
		FeedbackType: &feedbackType,
		ID:           id.String(),
	}))
}

func (r *EpicRepository) ClearEpicFeedback(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ClearEpicFeedback(ctx, id.String()))
}

func (r *EpicRepository) ReleaseEpicClaim(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ReleaseEpicClaim(ctx, id.String()))
}

func (r *EpicRepository) ListStaleEpics(ctx context.Context, threshold time.Time) ([]*epic.Epic, error) {
	thresholdUnix := threshold.Unix()
	rows, err := r.db.ListStaleEpics(ctx, &thresholdUnix)
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
	// SQLite doesn't support array_remove, so read-modify-write
	existing, err := r.db.ReadEpic(ctx, id.String())
	if err != nil {
		return tagEpicErr(err)
	}
	var taskIDs []string
	_ = json.Unmarshal([]byte(existing.TaskIds), &taskIDs)
	filtered := make([]string, 0, len(taskIDs))
	for _, tid := range taskIDs {
		if tid != taskID {
			filtered = append(filtered, tid)
		}
	}
	taskIDsJSON, _ := json.Marshal(filtered)
	return tagEpicErr(r.db.SetEpicTaskIDs(ctx, sqlc.SetEpicTaskIDsParams{
		TaskIds: string(taskIDsJSON),
		ID:      id.String(),
	}))
}

func unmarshalEpic(in *sqlc.Epic) *epic.Epic {
	e := &epic.Epic{
		ID:              epic.MustParseEpicID(in.ID),
		RepoID:          in.RepoID,
		Title:           in.Title,
		Description:     in.Description,
		Status:          epic.Status(in.Status),
		NotReady:        in.NotReady != 0,
		ClaimedAt:       unixPtrToTimePtr(in.ClaimedAt),
		LastHeartbeatAt: unixPtrToTimePtr(in.LastHeartbeatAt),
		Feedback:        in.Feedback,
		FeedbackType:    in.FeedbackType,
		CreatedAt:       unixToTime(in.CreatedAt),
		UpdatedAt:       unixToTime(in.UpdatedAt),
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
	_ = json.Unmarshal([]byte(in.ProposedTasks), &e.ProposedTasks)
	if e.ProposedTasks == nil {
		e.ProposedTasks = []epic.ProposedTask{}
	}
	_ = json.Unmarshal([]byte(in.TaskIds), &e.TaskIDs)
	if e.TaskIDs == nil {
		e.TaskIDs = []string{}
	}
	_ = json.Unmarshal([]byte(in.SessionLog), &e.SessionLog)
	if e.SessionLog == nil {
		e.SessionLog = []string{}
	}
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
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[epic.ErrTagEpicNotFound](err)
	}
	if isSQLiteErrCode(err, sqliteConstraint, sqliteConstraintUnique, sqliteConstraintPrimaryKey) {
		return errtag.Tag[epic.ErrTagEpicConflict](err)
	}
	return err
}
