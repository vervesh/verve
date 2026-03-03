package settingapi_test

import (
	"net/http"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/settingapi"
)

func TestGetDefaultModel_Default(t *testing.T) {
	f := newFixture(t)

	res := testutil.Get[server.Response[settingapi.DefaultModelResponse]](t, f.defaultModelURL())
	assert.Equal(t, "", res.Data.Model, "expected empty model when no default explicitly set")
}

func TestSaveDefaultModel(t *testing.T) {
	f := newFixture(t)

	req := settingapi.DefaultModelRequest{Model: "opus"}
	res := testutil.Put[server.Response[settingapi.DefaultModelResponse]](t, f.defaultModelURL(), req)
	assert.Equal(t, "opus", res.Data.Model)
	assert.True(t, res.Data.Configured)
}

func TestSaveDefaultModel_EmptyModel(t *testing.T) {
	f := newFixture(t)

	req := settingapi.DefaultModelRequest{Model: ""}
	httpReq, err := http.NewRequest(http.MethodPut, f.defaultModelURL(), mustJSONReader(req))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := testutil.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "expected validation error for empty model")
}

func TestGetGitHubTokenStatus_NotConfigured(t *testing.T) {
	f := newFixture(t)

	res := testutil.Get[server.Response[settingapi.GitHubTokenStatusResponse]](t, f.githubTokenURL())
	assert.False(t, res.Data.Configured, "expected configured=false when no token service")
}

func TestSaveGitHubToken_NoService(t *testing.T) {
	f := newFixture(t)

	req := settingapi.SaveGitHubTokenRequest{Token: "ghp_test"}
	httpReq, err := http.NewRequest(http.MethodPut, f.githubTokenURL(), mustJSONReader(req))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := testutil.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
}
