package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTask(t *testing.T) {
	tsk := NewTask("repo_123", "Fix bug", "Fix the login bug", nil, nil, 10.0, false, false, "sonnet", true)

	assert.NotEmpty(t, tsk.ID.String(), "expected non-empty ID")
	assert.Equal(t, "repo_123", tsk.RepoID)
	assert.Equal(t, "Fix bug", tsk.Title)
	assert.Equal(t, "Fix the login bug", tsk.Description)
	assert.Equal(t, StatusPending, tsk.Status)
	assert.Equal(t, 1, tsk.Attempt)
	assert.Equal(t, 5, tsk.MaxAttempts)
	assert.Equal(t, 10.0, tsk.MaxCostUSD)
	assert.False(t, tsk.SkipPR, "expected SkipPR false")
	assert.Equal(t, "sonnet", tsk.Model)
	assert.False(t, tsk.CreatedAt.IsZero(), "expected non-zero CreatedAt")
	assert.False(t, tsk.UpdatedAt.IsZero(), "expected non-zero UpdatedAt")
}

func TestNewTask_NilSlicesBecomEmpty(t *testing.T) {
	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)

	assert.NotNil(t, tsk.DependsOn, "expected DependsOn to be non-nil empty slice")
	assert.Len(t, tsk.DependsOn, 0)
	assert.NotNil(t, tsk.AcceptanceCriteria, "expected AcceptanceCriteria to be non-nil empty slice")
	assert.Len(t, tsk.AcceptanceCriteria, 0)
}

func TestNewTask_WithDependencies(t *testing.T) {
	deps := []string{"tsk_abc", "tsk_def"}
	criteria := []string{"Tests pass", "No regressions"}
	tsk := NewTask("repo_123", "title", "desc", deps, criteria, 5.0, true, false, "opus", true)

	assert.Len(t, tsk.DependsOn, 2)
	assert.Equal(t, "tsk_abc", tsk.DependsOn[0])
	assert.Len(t, tsk.AcceptanceCriteria, 2)
	assert.True(t, tsk.SkipPR, "expected SkipPR true")
	assert.Equal(t, "opus", tsk.Model)
}

func TestComputeDuration_NilStartedAt(t *testing.T) {
	tsk := &Task{Status: StatusRunning, StartedAt: nil}
	tsk.ComputeDuration()
	assert.Nil(t, tsk.DurationMs, "expected DurationMs to be nil when StartedAt is nil")
}

func TestComputeDuration_PendingStatus(t *testing.T) {
	now := time.Now()
	tsk := &Task{Status: StatusPending, StartedAt: &now}
	tsk.ComputeDuration()
	assert.Nil(t, tsk.DurationMs, "expected DurationMs to be nil for pending status")
}

func TestComputeDuration_RunningStatus(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	tsk := &Task{Status: StatusRunning, StartedAt: &start}
	tsk.ComputeDuration()
	require.NotNil(t, tsk.DurationMs, "expected DurationMs to be set for running status")
	assert.GreaterOrEqual(t, *tsk.DurationMs, int64(4000), "expected duration >= 4000ms")
}

func TestComputeDuration_CompletedStatuses(t *testing.T) {
	statuses := []Status{StatusReview, StatusMerged, StatusClosed, StatusFailed}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			start := time.Now().Add(-10 * time.Second)
			updated := time.Now().Add(-5 * time.Second)
			tsk := &Task{
				Status:    status,
				StartedAt: &start,
				UpdatedAt: updated,
			}
			tsk.ComputeDuration()
			require.NotNil(t, tsk.DurationMs, "expected DurationMs to be set for %s status", status)
			// Should be approximately 5 seconds (10s start - 5s updated)
			expected := updated.Sub(start).Milliseconds()
			assert.Equal(t, expected, *tsk.DurationMs)
		})
	}
}

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusReview, "review"},
		{StatusMerged, "merged"},
		{StatusClosed, "closed"},
		{StatusFailed, "failed"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, string(tt.status))
	}
}
