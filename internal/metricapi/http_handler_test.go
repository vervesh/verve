package metricapi_test

import (
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/joshjon/verve/internal/metric"
	"github.com/joshjon/verve/internal/task"
)

func TestGetMetrics_Empty(t *testing.T) {
	f := newFixture(t)

	res := testutil.Get[server.Response[metric.Metrics]](t, f.metricsURL())
	assert.Equal(t, 0, res.Data.TotalTasks)
	assert.Equal(t, 0, res.Data.RunningAgents)
	assert.Equal(t, 0, res.Data.PendingTasks)
	assert.Empty(t, res.Data.ActiveAgents)
	assert.Empty(t, res.Data.RecentCompletions)
}

func TestGetMetrics_WithTasks(t *testing.T) {
	f := newFixture(t)

	f.seedTask("Pending Task", task.StatusPending)
	f.seedTask("Running Task", task.StatusRunning)
	f.seedTask("Review Task", task.StatusReview)
	f.seedTask("Failed Task", task.StatusFailed)
	f.seedTask("Merged Task", task.StatusMerged)

	res := testutil.Get[server.Response[metric.Metrics]](t, f.metricsURL())
	assert.Equal(t, 5, res.Data.TotalTasks)
	assert.Equal(t, 1, res.Data.PendingTasks)
	assert.Equal(t, 1, res.Data.RunningAgents)
	assert.Equal(t, 1, res.Data.ReviewTasks)
	assert.Equal(t, 1, res.Data.FailedTasks)
	assert.Equal(t, 1, res.Data.CompletedTasks)
}
