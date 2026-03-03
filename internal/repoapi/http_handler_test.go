package repoapi_test

import (
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
