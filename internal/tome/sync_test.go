package tome_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

// setupGitRepos creates a bare remote repo and two clones for testing sync.
func setupGitRepos(t *testing.T) (remote, clone1, clone2 string) {
	t.Helper()

	base := t.TempDir()
	remote = filepath.Join(base, "remote.git")
	clone1 = filepath.Join(base, "clone1")
	clone2 = filepath.Join(base, "clone2")

	// Create bare remote with explicit main branch.
	run(t, "", "git", "init", "--bare", "--initial-branch=main", remote)

	// Create clone1 with an initial commit.
	run(t, "", "git", "init", "--initial-branch=main", clone1)
	gitConfig(t, clone1, "user1@example.com", "User One")
	run(t, clone1, "git", "remote", "add", "origin", remote)
	writeFile(t, filepath.Join(clone1, "README.md"), "# Test Repo\n")
	run(t, clone1, "git", "add", "README.md")
	run(t, clone1, "git", "commit", "-m", "initial commit")
	run(t, clone1, "git", "push", "-u", "origin", "main")

	// Create clone2.
	run(t, "", "git", "clone", remote, clone2)
	gitConfig(t, clone2, "user2@example.com", "User Two")

	return remote, clone1, clone2
}

func gitConfig(t *testing.T, dir, email, name string) {
	t.Helper()
	run(t, dir, "git", "config", "user.email", email)
	run(t, dir, "git", "config", "user.name", name)
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "command %s %v failed: %s", name, args, stderr.String())
	return stdout.String()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestSyncPushCreatesOrphanBranch(t *testing.T) {
	_, clone1, _ := setupGitRepos(t)
	tm := openTestTome(t)
	ctx := context.Background()

	// Record a session.
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary:   "Test session for sync",
		Learnings: "Learned something useful",
		Tags:      []string{"test"},
		User:      "userone",
	}))

	// Push to remote.
	result, err := tm.Sync(ctx, clone1, "userone", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Exported)
	assert.Equal(t, 0, result.Imported)

	// Verify the branch exists on the remote.
	out := run(t, clone1, "git", "branch", "-a")
	assert.Contains(t, out, "tome/context/userone")
}

func TestSyncPullImportsSessions(t *testing.T) {
	_, clone1, clone2 := setupGitRepos(t)
	ctx := context.Background()

	// Record and push from clone1.
	tm1 := openTestTome(t)
	require.NoError(t, tm1.Record(ctx, tome.Session{
		Summary:   "Session from user1",
		Learnings: "User1 learned this",
		Tags:      []string{"user1"},
		User:      "userone",
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}))
	_, err := tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)

	// Pull from clone2.
	tm2 := openTestTome(t)
	result, err := tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{PullOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)

	// Verify the session was imported.
	sessions, err := tm2.Log(ctx, 10, "")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "Session from user1", sessions[0].Summary)
	assert.Equal(t, "userone", sessions[0].User)
}

func TestSyncBidirectional(t *testing.T) {
	_, clone1, clone2 := setupGitRepos(t)
	ctx := context.Background()

	// User1 records and pushes.
	tm1 := openTestTome(t)
	require.NoError(t, tm1.Record(ctx, tome.Session{
		Summary: "Session from user1",
		User:    "userone",
	}))
	_, err := tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)

	// User2 records and pushes.
	tm2 := openTestTome(t)
	require.NoError(t, tm2.Record(ctx, tome.Session{
		Summary: "Session from user2",
		User:    "usertwo",
	}))
	_, err = tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)

	// User1 pulls — should get user2's session.
	result1, err := tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{PullOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Imported)

	// User2 pulls — should get user1's session.
	result2, err := tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{PullOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Imported)

	// Both should now have 2 sessions.
	sessions1, err := tm1.Log(ctx, 10, "")
	require.NoError(t, err)
	assert.Len(t, sessions1, 2)

	sessions2, err := tm2.Log(ctx, 10, "")
	require.NoError(t, err)
	assert.Len(t, sessions2, 2)
}

func TestSyncDeduplicates(t *testing.T) {
	_, clone1, clone2 := setupGitRepos(t)
	ctx := context.Background()

	// User1 pushes a session.
	tm1 := openTestTome(t)
	require.NoError(t, tm1.Record(ctx, tome.Session{
		Summary: "Shared session",
		User:    "userone",
	}))
	_, err := tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)

	// User2 pulls.
	tm2 := openTestTome(t)
	result, err := tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{PullOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)

	// User2 pulls again — should not re-import.
	result, err = tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{PullOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Imported)

	sessions, err := tm2.Log(ctx, 10, "")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestSyncExportedNotPushedAgain(t *testing.T) {
	_, clone1, _ := setupGitRepos(t)
	ctx := context.Background()

	tm := openTestTome(t)
	require.NoError(t, tm.Record(ctx, tome.Session{
		Summary: "Already pushed session",
		User:    "userone",
	}))

	// First push.
	result, err := tm.Sync(ctx, clone1, "userone", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Exported)

	// Second push — should export 0.
	result, err = tm.Sync(ctx, clone1, "userone", tome.SyncOpts{PushOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Exported)
}

func TestSyncFullRoundTrip(t *testing.T) {
	_, clone1, clone2 := setupGitRepos(t)
	ctx := context.Background()

	// User1 does full sync (pull + push) with one session.
	tm1 := openTestTome(t)
	require.NoError(t, tm1.Record(ctx, tome.Session{
		Summary:   "Session A",
		Learnings: "Learned A",
		Tags:      []string{"a"},
		Files:     []string{"a.go"},
		User:      "userone",
	}))
	result, err := tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Imported)
	assert.Equal(t, 1, result.Exported)

	// User2 does full sync — pulls user1's session, pushes own.
	tm2 := openTestTome(t)
	require.NoError(t, tm2.Record(ctx, tome.Session{
		Summary:   "Session B",
		Learnings: "Learned B",
		Tags:      []string{"b"},
		Files:     []string{"b.go"},
		User:      "usertwo",
	}))
	result, err = tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 1, result.Exported)

	// User1 full sync again — picks up user2's session.
	result, err = tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 0, result.Exported)

	// Verify user1's search finds both sessions.
	results, err := tm1.Search(ctx, "learned", tome.SearchOpts{BM25Only: true})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSyncCustomBranch(t *testing.T) {
	_, clone1, clone2 := setupGitRepos(t)
	ctx := context.Background()

	tm1 := openTestTome(t)
	require.NoError(t, tm1.Record(ctx, tome.Session{
		Summary: "Custom branch session",
		User:    "userone",
	}))

	// Push to custom branch.
	result, err := tm1.Sync(ctx, clone1, "userone", tome.SyncOpts{
		PushOnly: true,
		Branch:   "tome/context/shared",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Exported)

	// Pull from clone2 — should pick up the custom branch.
	tm2 := openTestTome(t)
	result, err = tm2.Sync(ctx, clone2, "usertwo", tome.SyncOpts{PullOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
}
