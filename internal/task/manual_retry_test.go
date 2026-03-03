package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManualRetryTask_PreservesPRInfo tests that retrying a failed task
// preserves the PR association (URL, number, and branch name).
func TestManualRetryTask_PreservesPRInfo(t *testing.T) {
	// Use a custom mock that simulates the actual SQL behavior
	repo := &mockRepoWithPRPreservation{
		mockRepository: newMockRepo(),
	}
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// Create a failed task with PR info
	tsk := NewTask("repo_123", "Fix bug", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusFailed
	tsk.PullRequestURL = "https://github.com/owner/repo/pull/42"
	tsk.PRNumber = 42
	tsk.BranchName = "verve/task-tsk_123"
	repo.tasks[tsk.ID.String()] = tsk

	// Retry the task
	err := store.ManualRetryTask(context.Background(), tsk.ID, "please try again")
	require.NoError(t, err)

	// Verify PR info is preserved
	updatedTask, err := store.ReadTask(context.Background(), tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, updatedTask.Status)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", updatedTask.PullRequestURL, "PR URL should be preserved")
	assert.Equal(t, 42, updatedTask.PRNumber, "PR number should be preserved")
	assert.Equal(t, "verve/task-tsk_123", updatedTask.BranchName, "Branch name should be preserved")
}

// mockRepoWithPRPreservation extends mockRepository to simulate the actual
// SQL behavior where ManualRetryTask preserves PR fields.
type mockRepoWithPRPreservation struct {
	*mockRepository
}

func (m *mockRepoWithPRPreservation) ManualRetryTask(ctx context.Context, id TaskID, instructions string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tasks[id.String()]
	if !ok || t.Status != StatusFailed {
		return false, nil
	}

	// Simulate the SQL update - preserve PR fields
	t.Status = StatusPending
	t.Attempt++
	t.RetryReason = instructions
	t.RetryContext = ""
	t.CloseReason = ""
	t.ConsecutiveFailures = 0
	t.StartedAt = nil
	t.UpdatedAt = time.Now()
	// PR fields are NOT cleared: PullRequestURL, PRNumber, BranchName

	return true, nil
}
