package tome_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

// seedCorpus inserts a set of sessions with overlapping vocabulary
// to give LSA meaningful co-occurrence patterns to learn from.
func seedCorpus(t *testing.T, tm *tome.Tome) {
	t.Helper()
	ctx := context.Background()

	sessions := []tome.Session{
		{
			Summary:   "Added JWT authentication middleware",
			Learnings: "Token validation in auth middleware using bearer scheme. Refresh tokens stored in httponly cookies.",
			Tags:      []string{"auth", "jwt", "middleware"},
			Files:     []string{"src/auth/middleware.go", "src/auth/tokens.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-10 * 24 * time.Hour),
		},
		{
			Summary:   "Implemented user login flow",
			Learnings: "Password hashing with bcrypt. Session cookies for login state. Auth redirect on expired session.",
			Tags:      []string{"auth", "login", "user"},
			Files:     []string{"src/auth/login.go", "src/user/handler.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-9 * 24 * time.Hour),
		},
		{
			Summary:   "Fixed authentication token refresh bug",
			Learnings: "Bearer token rotation required redis for blacklist. Auth middleware checks token expiry.",
			Tags:      []string{"auth", "jwt", "bugfix"},
			Files:     []string{"src/auth/refresh.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		},
		{
			Summary:   "Added rate limiter to API endpoints",
			Learnings: "Sliding window algorithm for rate limiting. Middleware chain executes rate check before handler.",
			Tags:      []string{"api", "rate-limiting", "middleware"},
			Files:     []string{"src/api/ratelimit.go", "src/api/middleware.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-7 * 24 * time.Hour),
		},
		{
			Summary:   "User session management improvements",
			Learnings: "Login session timeout set to 24 hours. Cookie secure flag required for production.",
			Tags:      []string{"user", "session"},
			Files:     []string{"src/user/session.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-6 * 24 * time.Hour),
		},
		{
			Summary:   "API error handling improvements",
			Learnings: "Consistent error response format across all API endpoints. Error middleware wraps handler errors.",
			Tags:      []string{"api", "error-handling"},
			Files:     []string{"src/api/errors.go", "src/api/middleware.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-5 * 24 * time.Hour),
		},
		{
			Summary:   "Database migration for user accounts",
			Learnings: "Schema migration adds email verification column. Foreign key constraints for user sessions table.",
			Tags:      []string{"database", "migration", "user"},
			Files:     []string{"migrations/003_user_accounts.sql"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-4 * 24 * time.Hour),
		},
		{
			Summary:   "OAuth2 provider integration",
			Learnings: "OAuth2 authorization code flow. Provider tokens stored encrypted. Auth callback validates state parameter.",
			Tags:      []string{"auth", "oauth2"},
			Files:     []string{"src/auth/oauth.go", "src/auth/callback.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-3 * 24 * time.Hour),
		},
		{
			Summary:   "Password reset email functionality",
			Learnings: "Reset token expires after 1 hour. Email template uses user verification link.",
			Tags:      []string{"user", "email"},
			Files:     []string{"src/user/reset.go", "src/email/templates.go"},
			Status:    "failed",
			CreatedAt: time.Now().Add(-2 * 24 * time.Hour),
		},
		{
			Summary:   "API versioning strategy",
			Learnings: "URL-based versioning with /v1/ prefix. Endpoint routing through version middleware.",
			Tags:      []string{"api", "versioning"},
			Files:     []string{"src/api/router.go"},
			Status:    "succeeded",
			CreatedAt: time.Now().Add(-1 * 24 * time.Hour),
		},
	}

	for _, s := range sessions {
		require.NoError(t, tm.Record(ctx, s))
	}
}

func TestBuildLSAIndex(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)

	numDocs, numTerms, dim, err := tm.BuildIndex(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 10, numDocs)
	assert.Greater(t, numTerms, 0)
	assert.Greater(t, dim, 0)
	assert.LessOrEqual(t, dim, 9) // min(128, numDocs-1)
}

func TestBuildLSATooFewSessions(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	// 0 sessions
	_, _, _, err := tm.BuildIndex(ctx)
	assert.Error(t, err)

	// 1 session
	require.NoError(t, tm.Record(ctx, tome.Session{Summary: "Only session", Learnings: "something"}))
	_, _, _, err = tm.BuildIndex(ctx)
	assert.Error(t, err)
}

func TestHybridSearchFindsResults(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)
	ctx := context.Background()

	// Hybrid search should return results
	results, err := tm.Search(ctx, "authentication middleware", tome.SearchOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// All results should have positive scores
	for _, r := range results {
		assert.Greater(t, r.Score, 0.0)
	}
}

func TestHybridSearchLSAFindsSemanticMatches(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)
	ctx := context.Background()

	// BM25-only search for "token" finds exact keyword matches
	bm25Results, err := tm.Search(ctx, "token", tome.SearchOpts{BM25Only: true})
	require.NoError(t, err)

	// Hybrid search should find at least as many results (BM25 + LSA)
	hybridResults, err := tm.Search(ctx, "token", tome.SearchOpts{})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(hybridResults), len(bm25Results),
		"hybrid search should find at least as many results as BM25-only")
}

func TestBM25OnlyFlag(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)
	ctx := context.Background()

	// BM25-only should work and return keyword matches
	results, err := tm.Search(ctx, "authentication", tome.SearchOpts{BM25Only: true})
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// Every result should contain the search term in its text
	for _, r := range results {
		text := r.Session.Summary + " " + r.Session.Learnings
		assert.Contains(t, text, "uthenticat") // substring that handles case
	}
}

func TestHybridSearchWithFilters(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)
	ctx := context.Background()

	// Status filter
	results, err := tm.Search(ctx, "user", tome.SearchOpts{Status: "failed"})
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, "failed", r.Session.Status)
	}

	// File filter
	results, err = tm.Search(ctx, "middleware", tome.SearchOpts{FilePattern: "src/api/"})
	require.NoError(t, err)
	for _, r := range results {
		hasMatch := false
		for _, f := range r.Session.Files {
			if contains(f, "src/api/") {
				hasMatch = true
				break
			}
		}
		assert.True(t, hasMatch, "result should have a file matching the filter: %v", r.Session.Files)
	}
}

func TestHybridSearchGracefulDegradation(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	// With 0 sessions, search returns empty (no error)
	results, err := tm.Search(ctx, "anything", tome.SearchOpts{})
	require.NoError(t, err)
	assert.Empty(t, results)

	// With 1 session, falls back to BM25
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Single session about testing",
		Learnings: "Test patterns discovered",
	}))
	results, err = tm.Search(ctx, "testing", tome.SearchOpts{})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestHybridSearchLimit(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)
	ctx := context.Background()

	results, err := tm.Search(ctx, "auth", tome.SearchOpts{Limit: 2})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 2)
}

func TestLSAIndexAutoRebuilds(t *testing.T) {
	tm := openTestTome(t)
	seedCorpus(t, tm)
	ctx := context.Background()

	// First search builds the index
	results1, err := tm.Search(ctx, "authentication", tome.SearchOpts{})
	require.NoError(t, err)
	count1 := len(results1)

	// Add a new session
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "New authentication feature",
		Learnings: "Two-factor authentication with TOTP codes",
		Tags:      []string{"auth", "2fa"},
	}))

	// Next search should rebuild index and include the new session
	results2, err := tm.Search(ctx, "authentication", tome.SearchOpts{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results2), count1)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
