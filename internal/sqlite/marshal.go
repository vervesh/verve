package sqlite

import (
	"encoding/json"
	"time"

	"github.com/joshjon/verve/internal/sqlite/sqlc"
	"github.com/joshjon/verve/internal/task"
)

func unmarshalTask(in *sqlc.Task) *task.Task {
	t := &task.Task{
		ID:                 task.MustParseTaskID(in.ID),
		RepoID:             in.RepoID,
		Title:              in.Title,
		Description:        in.Description,
		Status:             task.Status(in.Status),
		DependsOn:          unmarshalJSONStrings(in.DependsOn),
		AcceptanceCriteria: unmarshalJSONStrings(in.AcceptanceCriteriaList),
		CreatedAt:          unixToTime(in.CreatedAt),
		UpdatedAt:          unixToTime(in.UpdatedAt),
	}
	if in.PullRequestUrl != nil {
		t.PullRequestURL = *in.PullRequestUrl
	}
	if in.PrNumber != nil {
		t.PRNumber = int(*in.PrNumber)
	}
	if in.CloseReason != nil {
		t.CloseReason = *in.CloseReason
	}
	t.Attempt = int(in.Attempt)
	t.MaxAttempts = int(in.MaxAttempts)
	if in.RetryReason != nil {
		t.RetryReason = *in.RetryReason
	}
	if in.AgentStatus != nil {
		t.AgentStatus = *in.AgentStatus
	}
	if in.RetryContext != nil {
		t.RetryContext = *in.RetryContext
	}
	t.ConsecutiveFailures = int(in.ConsecutiveFailures)
	t.CostUSD = in.CostUsd
	if in.MaxCostUsd != nil {
		t.MaxCostUSD = *in.MaxCostUsd
	}
	t.SkipPR = in.SkipPr != 0
	t.DraftPR = in.DraftPr != 0
	t.Ready = in.Ready != 0
	if in.Model != nil {
		t.Model = *in.Model
	}
	if in.BranchName != nil {
		t.BranchName = *in.BranchName
	}
	if in.EpicID != nil {
		t.EpicID = *in.EpicID
	}
	t.StartedAt = unixPtrToTimePtr(in.StartedAt)
	t.ComputeDuration()
	return t
}

func unmarshalTaskList(in []*sqlc.Task) []*task.Task {
	out := make([]*task.Task, len(in))
	for i := range in {
		out[i] = unmarshalTask(in[i])
	}
	return out
}

func marshalJSONStrings(ss []string) string {
	if ss == nil {
		ss = []string{}
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func unmarshalJSONStrings(s string) []string {
	var ss []string
	_ = json.Unmarshal([]byte(s), &ss)
	if ss == nil {
		ss = []string{}
	}
	return ss
}

func unixToTime(secs int64) time.Time {
	return time.Unix(secs, 0).UTC()
}

func unixPtrToTimePtr(secs *int64) *time.Time {
	if secs == nil {
		return nil
	}
	t := unixToTime(*secs)
	return &t
}

func ptr[T any](v T) *T {
	return &v
}
