package metric

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/task"
)

// mockTaskLister implements TaskLister using an in-memory task slice.
type mockTaskLister struct {
	tasks []*task.Task
}

func (m *mockTaskLister) ListTasks(_ context.Context) ([]*task.Task, error) {
	return m.tasks, nil
}

func TestCompute_Empty(t *testing.T) {
	lister := &mockTaskLister{}
	metrics, err := Compute(context.Background(), lister, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 0, metrics.RunningAgents)
	assert.Equal(t, 0, metrics.PendingTasks)
	assert.Equal(t, 0, metrics.ReviewTasks)
	assert.Equal(t, 0, metrics.TotalTasks)
	assert.Equal(t, 0, metrics.CompletedTasks)
	assert.Equal(t, 0, metrics.FailedTasks)
	assert.Equal(t, 0.0, metrics.TotalCostUSD)
	assert.Empty(t, metrics.ActiveAgents)
	assert.Empty(t, metrics.RecentCompletions)
}

func TestCompute_Counts(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-10 * time.Minute)

	pending := task.NewTask("repo_1", "pending task", "desc", nil, nil, 0, false, false, "sonnet", true)
	pending.Status = task.StatusPending

	running := task.NewTask("repo_1", "running task", "desc", nil, nil, 0, false, false, "opus", true)
	running.Status = task.StatusRunning
	running.StartedAt = &startedAt
	running.CostUSD = 1.50
	running.Model = "opus"

	review := task.NewTask("repo_1", "review task", "desc", nil, nil, 0, false, false, "sonnet", true)
	review.Status = task.StatusReview
	review.CostUSD = 0.75

	merged := task.NewTask("repo_1", "merged task", "desc", nil, nil, 0, false, false, "sonnet", true)
	merged.Status = task.StatusMerged
	merged.CostUSD = 2.00
	merged.UpdatedAt = now.Add(-5 * time.Minute)

	closed := task.NewTask("repo_1", "closed task", "desc", nil, nil, 0, false, false, "sonnet", true)
	closed.Status = task.StatusClosed
	closed.UpdatedAt = now.Add(-3 * time.Minute)

	failed := task.NewTask("repo_1", "failed task", "desc", nil, nil, 0, false, false, "sonnet", true)
	failed.Status = task.StatusFailed
	failed.CostUSD = 0.50
	failed.UpdatedAt = now.Add(-1 * time.Minute)

	lister := &mockTaskLister{
		tasks: []*task.Task{pending, running, review, merged, closed, failed},
	}

	metrics, err := Compute(context.Background(), lister, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 6, metrics.TotalTasks)
	assert.Equal(t, 1, metrics.RunningAgents)
	assert.Equal(t, 1, metrics.PendingTasks)
	assert.Equal(t, 1, metrics.ReviewTasks)
	assert.Equal(t, 2, metrics.CompletedTasks) // merged + closed
	assert.Equal(t, 1, metrics.FailedTasks)
	assert.InDelta(t, 4.75, metrics.TotalCostUSD, 0.01)

	// Check active agents
	require.Len(t, metrics.ActiveAgents, 1)
	assert.Equal(t, running.ID.String(), metrics.ActiveAgents[0].TaskID)
	assert.Equal(t, "running task", metrics.ActiveAgents[0].TaskTitle)
	assert.Equal(t, "opus", metrics.ActiveAgents[0].Model)
	assert.True(t, metrics.ActiveAgents[0].RunningFor > 0)

	// Check recent completions: merged, closed, failed = 3 total
	assert.Len(t, metrics.RecentCompletions, 3)
	// Should be sorted by UpdatedAt descending
	assert.Equal(t, "failed", metrics.RecentCompletions[0].Status)
}

func TestCompute_IncludesPlanningEpics(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-10 * time.Minute)

	running := task.NewTask("repo_1", "running task", "desc", nil, nil, 0, false, false, "sonnet", true)
	running.Status = task.StatusRunning
	running.StartedAt = &startedAt

	lister := &mockTaskLister{tasks: []*task.Task{running}}

	// Set up a mock planning epic lister
	claimedAt := now.Add(-5 * time.Minute)
	epicLister := NewPlanningEpicListerFunc(
		func(ctx context.Context) ([]PlanningEpic, error) {
			return []PlanningEpic{
				{
					ID:        "epc_planning1",
					Title:     "Plan user auth feature",
					RepoID:    "repo_1",
					Model:     "opus",
					ClaimedAt: &claimedAt,
				},
			}, nil
		},
	)

	metrics, err := Compute(context.Background(), lister, epicLister, nil)
	require.NoError(t, err)

	// Running agents should include both the running task and the planning epic
	assert.Equal(t, 2, metrics.RunningAgents)
	require.Len(t, metrics.ActiveAgents, 2)

	// Find the planning agent in the list
	var planningAgent *ActiveAgent
	var taskAgent *ActiveAgent
	for i := range metrics.ActiveAgents {
		if metrics.ActiveAgents[i].IsPlanning {
			planningAgent = &metrics.ActiveAgents[i]
		} else {
			taskAgent = &metrics.ActiveAgents[i]
		}
	}

	// Verify task agent
	require.NotNil(t, taskAgent)
	assert.Equal(t, running.ID.String(), taskAgent.TaskID)
	assert.False(t, taskAgent.IsPlanning)

	// Verify planning agent
	require.NotNil(t, planningAgent)
	assert.Equal(t, "epc_planning1", planningAgent.TaskID)
	assert.Equal(t, "Plan user auth feature", planningAgent.TaskTitle)
	assert.Equal(t, "repo_1", planningAgent.RepoID)
	assert.Equal(t, "opus", planningAgent.Model)
	assert.Equal(t, "epc_planning1", planningAgent.EpicID)
	assert.True(t, planningAgent.IsPlanning)
	assert.Equal(t, "Plan user auth feature", planningAgent.EpicTitle)
	assert.True(t, planningAgent.RunningFor > 0)

	// TotalTasks should only count actual tasks, not epics
	assert.Equal(t, 1, metrics.TotalTasks)
}

func TestCompute_NilEpicLister(t *testing.T) {
	lister := &mockTaskLister{}

	// No epic lister — should still work fine
	metrics, err := Compute(context.Background(), lister, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 0, metrics.RunningAgents)
	assert.Empty(t, metrics.ActiveAgents)
}

func TestCompute_RecentCompletionsLimit(t *testing.T) {
	now := time.Now()
	var tasks []*task.Task
	// Create 15 completed tasks
	for i := 0; i < 15; i++ {
		tsk := task.NewTask("repo_1", "task", "desc", nil, nil, 0, false, false, "sonnet", true)
		tsk.Status = task.StatusMerged
		tsk.UpdatedAt = now.Add(-time.Duration(i) * time.Minute)
		tasks = append(tasks, tsk)
	}

	lister := &mockTaskLister{tasks: tasks}
	metrics, err := Compute(context.Background(), lister, nil, nil)
	require.NoError(t, err)

	// Should be limited to 10
	assert.Len(t, metrics.RecentCompletions, 10)
}
