package repo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
)

func TestNewRepo_Valid(t *testing.T) {
	r, err := repo.NewRepo("octocat/hello-world")
	require.NoError(t, err)
	assert.Equal(t, "octocat", r.Owner)
	assert.Equal(t, "hello-world", r.Name)
	assert.Equal(t, "octocat/hello-world", r.FullName)
	assert.NotEmpty(t, r.ID.String(), "expected non-empty ID")
	assert.True(t, strings.HasPrefix(r.ID.String(), "repo_"), "expected repo_ prefix, got %s", r.ID.String())
	assert.False(t, r.CreatedAt.IsZero(), "expected non-zero CreatedAt")
}

func TestNewRepo_InvalidNoSlash(t *testing.T) {
	_, err := repo.NewRepo("justname")
	assert.Error(t, err, "expected error for repo name without slash")
}

func TestNewRepo_EmptyOwner(t *testing.T) {
	_, err := repo.NewRepo("/reponame")
	assert.Error(t, err, "expected error for empty owner")
}

func TestNewRepo_EmptyName(t *testing.T) {
	_, err := repo.NewRepo("owner/")
	assert.Error(t, err, "expected error for empty name")
}

func TestNewRepo_EmptyString(t *testing.T) {
	_, err := repo.NewRepo("")
	assert.Error(t, err, "expected error for empty string")
}

func TestNewRepo_MultipleSlashes(t *testing.T) {
	r, err := repo.NewRepo("owner/repo/subpath")
	require.NoError(t, err)
	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "repo/subpath", r.Name)
}

// --- RepoID tests ---

func TestNewRepoID(t *testing.T) {
	id := repo.NewRepoID()
	s := id.String()
	assert.NotEmpty(t, s, "expected non-empty string")
	assert.True(t, strings.HasPrefix(s, "repo_"), "expected repo_ prefix, got %s", s)
}

func TestParseRepoID_Valid(t *testing.T) {
	original := repo.NewRepoID()
	parsed, err := repo.ParseRepoID(original.String())
	require.NoError(t, err)
	assert.Equal(t, original.String(), parsed.String())
}

func TestParseRepoID_InvalidPrefix(t *testing.T) {
	_, err := repo.ParseRepoID("tsk_01h2xcejqtf2nbrexx3vqjhp41")
	assert.Error(t, err, "expected error for wrong prefix")
}

func TestParseRepoID_Empty(t *testing.T) {
	_, err := repo.ParseRepoID("")
	assert.Error(t, err, "expected error for empty string")
}

func TestMustParseRepoID_Panics(t *testing.T) {
	assert.Panics(t, func() {
		repo.MustParseRepoID("invalid")
	}, "expected panic for invalid repo ID")
}

// --- Store tests (backed by real SQLite) ---

func newTestStore(t *testing.T) *repo.Store {
	t.Helper()
	db := sqlite.NewTestDB(t)
	repoRepo := sqlite.NewRepoRepository(db)
	return repo.NewStore(repoRepo)
}

func TestStore_DeleteRepo(t *testing.T) {
	store := newTestStore(t)

	r, _ := repo.NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	err := store.DeleteRepo(context.Background(), r.ID)
	require.NoError(t, err)
}

func TestStore_CreateAndReadRepo(t *testing.T) {
	store := newTestStore(t)

	r, _ := repo.NewRepo("owner/name")
	err := store.CreateRepo(context.Background(), r)
	require.NoError(t, err)

	read, err := store.ReadRepo(context.Background(), r.ID)
	require.NoError(t, err)
	assert.Equal(t, r.FullName, read.FullName)
	assert.Equal(t, repo.SetupStatusPending, read.SetupStatus)
	assert.Empty(t, read.TechStack)
}

func TestStore_ListRepos(t *testing.T) {
	store := newTestStore(t)

	r1, _ := repo.NewRepo("owner/repo1")
	r2, _ := repo.NewRepo("owner/repo2")
	_ = store.CreateRepo(context.Background(), r1)
	_ = store.CreateRepo(context.Background(), r2)

	repos, err := store.ListRepos(context.Background())
	require.NoError(t, err)
	assert.Len(t, repos, 2)
}

func TestStore_ReadRepoByFullName(t *testing.T) {
	store := newTestStore(t)

	r, _ := repo.NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	read, err := store.ReadRepoByFullName(context.Background(), "owner/name")
	require.NoError(t, err)
	assert.Equal(t, r.ID.String(), read.ID.String())
}

func TestStore_UpdateRepoSetupScan(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	// Transition pending -> scanning first
	require.NoError(t, store.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))

	// Now scan complete -> needs_setup
	result := repo.SetupScanResult{
		Summary:     "A Go web application",
		TechStack:   []string{"Go", "PostgreSQL"},
		HasCode:     true,
		HasCLAUDEMD: true,
		HasREADME:   false,
		SetupStatus: repo.SetupStatusNeedsSetup,
	}
	require.NoError(t, store.UpdateRepoSetupScan(ctx, r.ID, result))

	read, err := store.ReadRepo(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "A Go web application", read.Summary)
	assert.Equal(t, []string{"Go", "PostgreSQL"}, read.TechStack)
	assert.True(t, read.HasCode)
	assert.True(t, read.HasCLAUDEMD)
	assert.False(t, read.HasREADME)
	assert.Equal(t, repo.SetupStatusNeedsSetup, read.SetupStatus)
}

func TestStore_UpdateRepoSetupScan_InvalidTransition(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	// Cannot go from pending -> needs_setup directly
	result := repo.SetupScanResult{
		SetupStatus: repo.SetupStatusNeedsSetup,
	}
	err := store.UpdateRepoSetupScan(ctx, r.ID, result)
	assert.Error(t, err)
}

func TestStore_UpdateRepoSetupStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	// Valid: pending -> scanning
	require.NoError(t, store.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))

	read, err := store.ReadRepo(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, repo.SetupStatusScanning, read.SetupStatus)
}

func TestStore_UpdateRepoSetupStatus_InvalidTransition(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	// Invalid: pending -> needs_setup (must go through scanning first)
	err := store.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusNeedsSetup)
	assert.Error(t, err)
}

func TestStore_UpdateRepoSetupStatus_InvalidStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	err := store.UpdateRepoSetupStatus(ctx, r.ID, "bogus")
	assert.Error(t, err)
}

func TestStore_UpdateRepoExpectations(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	now := time.Now().Truncate(time.Second)
	update := repo.ExpectationsUpdate{
		Expectations:     "Use conventional commits",
		SetupCompletedAt: &now,
	}
	require.NoError(t, store.UpdateRepoExpectations(ctx, r.ID, update))

	read, err := store.ReadRepo(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "Use conventional commits", read.Expectations)
	require.NotNil(t, read.SetupCompletedAt)
	assert.Equal(t, now.Unix(), read.SetupCompletedAt.Unix())
}

func TestStore_ListReposBySetupStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r1, _ := repo.NewRepo("owner/repo1")
	r2, _ := repo.NewRepo("owner/repo2")
	require.NoError(t, store.CreateRepo(ctx, r1))
	require.NoError(t, store.CreateRepo(ctx, r2))

	// Both start as pending
	repos, err := store.ListReposBySetupStatus(ctx, repo.SetupStatusPending)
	require.NoError(t, err)
	assert.Len(t, repos, 2)

	// Transition one to scanning
	require.NoError(t, store.UpdateRepoSetupStatus(ctx, r1.ID, repo.SetupStatusScanning))

	repos, err = store.ListReposBySetupStatus(ctx, repo.SetupStatusPending)
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, r2.ID.String(), repos[0].ID.String())
}

func TestStore_NewRepoDefaults(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	r, _ := repo.NewRepo("owner/name")
	require.NoError(t, store.CreateRepo(ctx, r))

	read, err := store.ReadRepo(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, repo.SetupStatusPending, read.SetupStatus)
	assert.Empty(t, read.Summary)
	assert.Empty(t, read.TechStack)
	assert.False(t, read.HasCode)
	assert.False(t, read.HasCLAUDEMD)
	assert.False(t, read.HasREADME)
	assert.Empty(t, read.Expectations)
	assert.Nil(t, read.SetupCompletedAt)
}
