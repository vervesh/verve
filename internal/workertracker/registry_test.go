package workertracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordPollStart(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(r *Registry)
		workerID      string
		maxConcurrent int
		activeTasks   int
		wantNew       bool
	}{
		{
			name:          "new worker registration",
			setup:         func(_ *Registry) {},
			workerID:      "worker-1",
			maxConcurrent: 4,
			activeTasks:   1,
			wantNew:       true,
		},
		{
			name: "update existing worker",
			setup: func(r *Registry) {
				r.RecordPollStart("worker-1", 2, 0)
				r.RecordPollEnd("worker-1")
			},
			workerID:      "worker-1",
			maxConcurrent: 8,
			activeTasks:   3,
			wantNew:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			tt.setup(r)

			r.RecordPollStart(tt.workerID, tt.maxConcurrent, tt.activeTasks)

			workers := r.ListWorkers(time.Minute)
			require.Len(t, workers, 1)
			assert.Equal(t, tt.workerID, workers[0].WorkerID)
			assert.Equal(t, tt.maxConcurrent, workers[0].MaxConcurrentTasks)
			assert.Equal(t, tt.activeTasks, workers[0].ActiveTasks)
			assert.True(t, workers[0].Polling)
		})
	}
}

func TestRecordPollEnd(t *testing.T) {
	t.Run("marks worker as not polling", func(t *testing.T) {
		r := New()
		r.RecordPollStart("worker-1", 4, 0)
		r.RecordPollEnd("worker-1")

		workers := r.ListWorkers(time.Minute)
		require.Len(t, workers, 1)
		assert.False(t, workers[0].Polling)
	})

	t.Run("no-op for unknown worker", func(t *testing.T) {
		r := New()
		r.RecordPollEnd("nonexistent")
		workers := r.ListWorkers(time.Minute)
		assert.Empty(t, workers)
	})
}

func TestListWorkers(t *testing.T) {
	t.Run("returns active workers", func(t *testing.T) {
		r := New()
		r.RecordPollStart("worker-1", 4, 1)
		r.RecordPollStart("worker-2", 2, 0)

		workers := r.ListWorkers(time.Minute)
		assert.Len(t, workers, 2)
	})

	t.Run("prunes stale workers", func(t *testing.T) {
		r := New()
		r.RecordPollStart("stale-worker", 1, 0)

		// Manually backdate the last poll time to make the worker stale.
		r.mu.Lock()
		entry := r.workers["stale-worker"]
		entry.info.LastPollAt = time.Now().Add(-5 * time.Minute)
		r.mu.Unlock()

		workers := r.ListWorkers(time.Minute)
		assert.Empty(t, workers, "stale worker should be pruned")
	})

	t.Run("computes UptimeMs and Polling", func(t *testing.T) {
		r := New()
		r.RecordPollStart("worker-1", 1, 0)

		// Small sleep to ensure non-zero uptime.
		time.Sleep(5 * time.Millisecond)

		workers := r.ListWorkers(time.Minute)
		require.Len(t, workers, 1)
		assert.True(t, workers[0].Polling)
		assert.Greater(t, workers[0].UptimeMs, int64(0))
	})

	t.Run("empty when no workers registered", func(t *testing.T) {
		r := New()
		workers := r.ListWorkers(time.Minute)
		assert.Empty(t, workers)
	})
}
