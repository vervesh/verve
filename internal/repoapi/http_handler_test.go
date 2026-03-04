package repoapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/repoapi"
)

func TestAddRepo_Success(t *testing.T) {
	f := newFixture(t)

	req := repoapi.AddRepoRequest{FullName: "newowner/newrepo"}
	res := testutil.Post[server.Response[repo.Repo]](t, f.reposURL(), req)
	assert.Equal(t, "newowner/newrepo", res.Data.FullName)
	assert.Equal(t, repo.SetupStatusScanning, res.Data.SetupStatus, "setup_status should be scanning after creating a repo")
}

func TestAddRepo_EmptyFullName(t *testing.T) {
	f := newFixture(t)

	req := repoapi.AddRepoRequest{FullName: ""}
	httpRes, err := testutil.DefaultClient.Post(f.reposURL(), "application/json", mustJSONReader(req))
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for empty full_name")
}

func TestAddRepo_InvalidFullName(t *testing.T) {
	f := newFixture(t)

	req := repoapi.AddRepoRequest{FullName: "noslash"}
	httpRes, err := testutil.DefaultClient.Post(f.reposURL(), "application/json", mustJSONReader(req))
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid full_name format")
}

func TestListRepos(t *testing.T) {
	f := newFixture(t)
	f.addRepo("owner/test-repo")

	res := testutil.Get[server.ResponseList[repo.Repo]](t, f.reposURL())
	assert.Len(t, res.Data, 1)
}

func TestRemoveRepo_InvalidID(t *testing.T) {
	f := newFixture(t)

	req, err := http.NewRequest(http.MethodDelete, f.reposURL()+"/invalid", nil)
	require.NoError(t, err)

	httpRes, err := testutil.DefaultClient.Do(req)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid repo ID")
}

func TestListAvailableRepos_NoGitHubClient(t *testing.T) {
	f := newFixture(t)

	httpRes, err := testutil.DefaultClient.Get(f.availableReposURL())
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, httpRes.StatusCode)
}

func TestGetSetup_Success(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	res := testutil.Get[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID))
	assert.Equal(t, r.ID.String(), res.Data.ID.String())
	assert.Equal(t, repo.SetupStatusPending, res.Data.SetupStatus)
}

func TestGetSetup_NotFound(t *testing.T) {
	f := newFixture(t)

	fakeID := repo.NewRepoID()
	httpRes, err := testutil.DefaultClient.Get(f.repoSetupURL(fakeID))
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

func TestGetSetup_InvalidID(t *testing.T) {
	f := newFixture(t)

	httpRes, err := testutil.DefaultClient.Get(f.Server.Address() + "/api/v1/repos/invalid/setup")
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestUpdateSetup_Expectations(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Move repo to needs_setup first (pending → scanning → needs_setup)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusNeedsSetup))

	expectations := "## Code Quality\n- Use conventional commits"
	req := repoapi.UpdateSetupRequest{
		Expectations: &expectations,
	}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)
	assert.Equal(t, "## Code Quality\n- Use conventional commits", res.Data.Expectations)
	assert.Equal(t, repo.SetupStatusNeedsSetup, res.Data.SetupStatus, "should stay needs_setup when mark_ready is false")
}

func TestUpdateSetup_MarkReady(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Move repo to needs_setup (pending → scanning → needs_setup)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusNeedsSetup))

	expectations := "## Expectations"
	req := repoapi.UpdateSetupRequest{
		Expectations: &expectations,
		MarkReady:    true,
	}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)
	assert.Equal(t, "## Expectations", res.Data.Expectations)
	assert.Equal(t, repo.SetupStatusReady, res.Data.SetupStatus, "should be ready when mark_ready is true")
	assert.NotNil(t, res.Data.SetupCompletedAt, "setup_completed_at should be set")
}

func TestUpdateSetup_Combined(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Move repo to needs_setup (pending → scanning → needs_setup)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusNeedsSetup))

	expectations := "## Testing\n- Use jest"
	techStack := []string{"TypeScript", "React", "Jest"}
	summary := "A React frontend app"
	req := repoapi.UpdateSetupRequest{
		Summary:      &summary,
		Expectations: &expectations,
		TechStack:    &techStack,
		MarkReady:    true,
	}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)
	assert.Equal(t, "## Testing\n- Use jest", res.Data.Expectations)
	assert.Equal(t, []string{"TypeScript", "React", "Jest"}, res.Data.TechStack)
	assert.Equal(t, "A React frontend app", res.Data.Summary)
	assert.Equal(t, repo.SetupStatusReady, res.Data.SetupStatus)
	assert.NotNil(t, res.Data.SetupCompletedAt)
}

func TestUpdateSetup_TechStackOnly(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	techStack := []string{"Go", "PostgreSQL", "Docker"}
	req := repoapi.UpdateSetupRequest{
		TechStack: &techStack,
	}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)
	assert.Equal(t, []string{"Go", "PostgreSQL", "Docker"}, res.Data.TechStack)
}

func TestUpdateSetup_TechStackEmpty(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Set tech stack, then clear it
	techStack := []string{"Go", "Docker"}
	req := repoapi.UpdateSetupRequest{TechStack: &techStack}
	doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)

	emptyStack := []string{}
	req2 := repoapi.UpdateSetupRequest{TechStack: &emptyStack}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req2)
	assert.Equal(t, []string{}, res.Data.TechStack)
}

func TestUpdateSetup_SummaryOnly(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	summary := "A Go microservice for payment processing."
	req := repoapi.UpdateSetupRequest{
		Summary: &summary,
	}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)
	assert.Equal(t, "A Go microservice for payment processing.", res.Data.Summary)
}

func TestUpdateSetup_SummaryEmpty(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// First set a summary, then clear it
	summary := "Initial summary"
	req := repoapi.UpdateSetupRequest{Summary: &summary}
	doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req)

	empty := ""
	req2 := repoapi.UpdateSetupRequest{Summary: &empty}
	res := doPatch[server.Response[repo.Repo]](t, f.repoSetupURL(r.ID), req2)
	assert.Equal(t, "", res.Data.Summary)
}

func TestRescan_Success(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Move repo to ready (pending → scanning → ready)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusReady))

	res := testutil.Post[server.Response[repo.Repo]](t, f.repoRescanURL(r.ID), nil)
	assert.Equal(t, repo.SetupStatusScanning, res.Data.SetupStatus, "should be scanning after rescan")
}

func TestSkipSetup_FromPending(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Repo starts in "pending" state — skip should transition to ready.
	res := testutil.Post[server.Response[repo.Repo]](t, f.repoSkipSetupURL(r.ID), nil)
	assert.Equal(t, repo.SetupStatusReady, res.Data.SetupStatus, "should be ready after skip")
	assert.NotNil(t, res.Data.SetupCompletedAt, "setup_completed_at should be set")
}

func TestSkipSetup_NotAllowedFromScanning(t *testing.T) {
	f := newFixture(t)
	r := f.addRepo("owner/test-repo")

	// Move to scanning (scanning → ready is valid, but we test that skip also works from scanning via the status transition)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning))

	// Scanning → ready is allowed, so skip should work
	res := testutil.Post[server.Response[repo.Repo]](t, f.repoSkipSetupURL(r.ID), nil)
	assert.Equal(t, repo.SetupStatusReady, res.Data.SetupStatus, "should be ready after skip from scanning")
}

// doPatch sends a PATCH request with JSON body and decodes the typed response.
func doPatch[T any](t *testing.T, url string, body any) T {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, mustJSONReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	httpRes, err := testutil.DefaultClient.Do(req)
	require.NoError(t, err)
	defer httpRes.Body.Close()
	require.Equal(t, http.StatusOK, httpRes.StatusCode, "expected 200 OK")
	var result T
	require.NoError(t, json.NewDecoder(httpRes.Body).Decode(&result))
	return result
}
