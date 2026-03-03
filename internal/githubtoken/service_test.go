package githubtoken_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/crypto"
	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/sqlite"
)

func validKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func newTestService(t *testing.T) *githubtoken.Service {
	t.Helper()
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewGitHubTokenRepository(db)
	return githubtoken.NewService(repo, validKey(), false)
}

func newTestServiceAndRepo(t *testing.T) (*githubtoken.Service, *sqlite.GitHubTokenRepository) {
	t.Helper()
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewGitHubTokenRepository(db)
	svc := githubtoken.NewService(repo, validKey(), false)
	return svc, repo
}

func TestIsValidTokenPrefix(t *testing.T) {
	tests := []struct {
		token string
		valid bool
	}{
		{"ghp_abc123", true},
		{"github_pat_abc123", true},
		{"gho_abc123", false},
		{"invalid", false},
		{"", false},
		{"ghp_", true},
		{"github_pat_", true},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := githubtoken.IsValidTokenPrefix(tt.token)
			assert.Equal(t, tt.valid, result, "IsValidTokenPrefix(%q)", tt.token)
		})
	}
}

func TestService_SaveAndGetToken(t *testing.T) {
	svc := newTestService(t)

	err := svc.SaveToken(context.Background(), "ghp_testtoken123")
	require.NoError(t, err, "save")

	token := svc.GetToken()
	assert.Equal(t, "ghp_testtoken123", token)

	assert.True(t, svc.HasToken(), "expected HasToken to return true")
	assert.False(t, svc.IsFineGrained(), "expected IsFineGrained to return false for ghp_ token")
}

func TestService_SaveFineGrainedToken(t *testing.T) {
	svc := newTestService(t)

	err := svc.SaveToken(context.Background(), "github_pat_testtoken123")
	require.NoError(t, err, "save")

	assert.True(t, svc.IsFineGrained(), "expected IsFineGrained to return true for github_pat_ token")
}

func TestService_GetClient(t *testing.T) {
	svc := newTestService(t)

	// Before save, client should be nil
	assert.Nil(t, svc.GetClient(), "expected nil client before save")

	_ = svc.SaveToken(context.Background(), "ghp_testtoken123")

	client := svc.GetClient()
	assert.NotNil(t, client, "expected non-nil client after save")
}

func TestService_DeleteToken(t *testing.T) {
	svc := newTestService(t)

	_ = svc.SaveToken(context.Background(), "ghp_testtoken123")
	require.True(t, svc.HasToken(), "expected token to be saved")

	err := svc.DeleteToken(context.Background())
	require.NoError(t, err, "delete")

	assert.False(t, svc.HasToken(), "expected HasToken to return false after delete")
	assert.Empty(t, svc.GetToken(), "expected empty token after delete")
	assert.Nil(t, svc.GetClient(), "expected nil client after delete")
}

func TestService_Load(t *testing.T) {
	_, repo := newTestServiceAndRepo(t)
	key := validKey()

	// Save a token via the repo directly (simulating a pre-existing token in DB)
	encrypted, err := crypto.Encrypt(key, "ghp_loaded_token")
	require.NoError(t, err, "encrypt")
	require.NoError(t, repo.UpsertGitHubToken(context.Background(), encrypted, time.Now()))

	// Create a fresh service and load from the DB
	svc := githubtoken.NewService(repo, key, false)
	err = svc.Load(context.Background())
	require.NoError(t, err, "load")

	assert.Equal(t, "ghp_loaded_token", svc.GetToken())
	assert.True(t, svc.HasToken(), "expected HasToken to return true after load")
}

func TestService_Load_NoToken(t *testing.T) {
	svc := newTestService(t)

	err := svc.Load(context.Background())
	require.NoError(t, err, "load with no token should not error")

	assert.False(t, svc.HasToken(), "expected HasToken to return false when no token stored")
}
