package tome_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

func openTestTome(t *testing.T) *tome.Tome {
	t.Helper()
	dir := t.TempDir()
	tm, err := tome.Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { tm.Close() })
	return tm
}

func TestRecordAndSearch(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	err := tm.Record(ctx, tome.Session{
		Summary:   "Added JWT refresh tokens",
		Learnings: "Redis required for token blacklist. Bearer auth expected.",
		Tags:      []string{"auth", "jwt"},
		Files:     []string{"src/auth.go", "src/tokens.go"},
		Status:    "succeeded",
	})
	require.NoError(t, err)

	results, err := tm.Search(ctx, "JWT", tome.SearchOpts{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Added JWT refresh tokens", results[0].Session.Summary)
	assert.Contains(t, results[0].Session.Learnings, "Redis required")
	assert.Equal(t, []string{"auth", "jwt"}, results[0].Session.Tags)
	assert.Equal(t, []string{"src/auth.go", "src/tokens.go"}, results[0].Session.Files)
}

func TestSearchMultipleResults(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	sessions := []tome.Session{
		{
			Summary:   "Added JWT refresh tokens",
			Learnings: "Redis required for token blacklist",
			Tags:      []string{"auth", "jwt"},
			Files:     []string{"src/auth.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-2 * 24 * time.Hour),
		},
		{
			Summary:   "Fixed rate limiter for API endpoints",
			Learnings: "Sliding window algorithm, configured per-route",
			Tags:      []string{"api", "rate-limiting"},
			Files:     []string{"src/api/middleware.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-5 * 24 * time.Hour),
		},
		{
			Summary:   "Implemented auth middleware",
			Learnings: "Auth middleware expects Bearer scheme in Authorization header",
			Tags:      []string{"auth", "middleware"},
			Files:     []string{"src/auth/middleware.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-1 * 24 * time.Hour),
		},
	}

	for _, s := range sessions {
		require.NoError(t, tm.Record(ctx, s))
	}

	// Search for "auth" should return auth-related sessions
	results, err := tm.Search(ctx, "auth", tome.SearchOpts{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
}

func TestSearchWithStatusFilter(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Added feature successfully",
		Learnings: "Database migration required",
		Status:    "succeeded",
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Failed to add feature",
		Learnings: "Database connection timeout",
		Status:    "failed",
	}))

	results, err := tm.Search(ctx, "database", tome.SearchOpts{Status: "succeeded"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "succeeded", results[0].Session.Status)
}

func TestSearchWithFileFilter(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Updated auth module",
		Learnings: "Token refresh logic",
		Files:     []string{"src/auth/tokens.go"},
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Updated API handler",
		Learnings: "Handler error handling",
		Files:     []string{"src/api/handler.go"},
	}))

	results, err := tm.Search(ctx, "updated", tome.SearchOpts{FilePattern: "src/auth/"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Updated auth module", results[0].Session.Summary)
}

func TestSearchNoResults(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	results, err := tm.Search(ctx, "nonexistent", tome.SearchOpts{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchLimit(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	for i := range 10 {
		require.NoError(t, tm.Record(ctx, tome.Session{
			Summary:   fmt.Sprintf("Session about testing %d", i),
			Learnings: "Testing patterns",
		}))
	}

	results, err := tm.Search(ctx, "testing", tome.SearchOpts{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestLog(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "First session",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Second session",
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Third session",
		CreatedAt: time.Now(),
	}))

	sessions, err := tm.Log(ctx, 10, "")
	require.NoError(t, err)
	require.Len(t, sessions, 3)
	// Most recent first
	assert.Equal(t, "Third session", sessions[0].Summary)
	assert.Equal(t, "Second session", sessions[1].Summary)
	assert.Equal(t, "First session", sessions[2].Summary)
}

func TestLogLimit(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	for i := range 10 {
		require.NoError(t, tm.Record(ctx, tome.Session{
			Summary:   fmt.Sprintf("Session %d", i),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}))
	}

	sessions, err := tm.Log(ctx, 3, "")
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
}

func TestRecordAutoFields(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	err := tm.Record(ctx, tome.Session{
		Summary: "Minimal session",
	})
	require.NoError(t, err)

	sessions, err := tm.Log(ctx, 1, "")
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	s := sessions[0]
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "Minimal session", s.Summary)
	assert.Equal(t, "succeeded", s.Status)
	assert.False(t, s.CreatedAt.IsZero())
	assert.Equal(t, []string{}, s.Tags)
	assert.Equal(t, []string{}, s.Files)
}

func TestSearchFiltersByRepo(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Auth work in puntr",
		Learnings: "JWT tokens for auth",
		Repo:      "joshjon/puntr",
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Auth work in verve",
		Learnings: "API keys for auth",
		Repo:      "joshjon/verve",
	}))

	// Search scoped to puntr.
	results, err := tm.Search(ctx, "auth", tome.SearchOpts{Repo: "joshjon/puntr"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Auth work in puntr", results[0].Session.Summary)

	// Search without repo filter returns both.
	results, err = tm.Search(ctx, "auth", tome.SearchOpts{})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestLogFiltersByRepo(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Session in repo A",
		Repo:      "owner/repo-a",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Session in repo B",
		Repo:      "owner/repo-b",
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}))
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Another in repo A",
		Repo:      "owner/repo-a",
		CreatedAt: time.Now(),
	}))

	// Log scoped to repo A.
	sessions, err := tm.Log(ctx, 10, "owner/repo-a")
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	assert.Equal(t, "Another in repo A", sessions[0].Summary)
	assert.Equal(t, "Session in repo A", sessions[1].Summary)

	// Log without repo filter returns all.
	sessions, err = tm.Log(ctx, 10, "")
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
}
