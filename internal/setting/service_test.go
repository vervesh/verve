package setting_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/setting"
	"github.com/joshjon/verve/internal/sqlite"
)

func newTestSettingService(t *testing.T) *setting.Service {
	t.Helper()
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewSettingRepository(db)
	return setting.NewService(repo)
}

func TestService_GetEmpty(t *testing.T) {
	svc := newTestSettingService(t)

	val := svc.Get("nonexistent")
	assert.Empty(t, val)
}

func TestService_SetAndGet(t *testing.T) {
	svc := newTestSettingService(t)
	ctx := context.Background()

	err := svc.Set(ctx, setting.KeyDefaultModel, "opus")
	require.NoError(t, err)

	val := svc.Get(setting.KeyDefaultModel)
	assert.Equal(t, "opus", val)
}

func TestService_Delete(t *testing.T) {
	svc := newTestSettingService(t)
	ctx := context.Background()

	_ = svc.Set(ctx, "key", "value")

	err := svc.Delete(ctx, "key")
	require.NoError(t, err)

	val := svc.Get("key")
	assert.Empty(t, val, "expected empty string after delete")
}

func TestService_Load(t *testing.T) {
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewSettingRepository(db)

	// Pre-populate settings directly via repo
	require.NoError(t, repo.UpsertSetting(context.Background(), "key1", "value1"))
	require.NoError(t, repo.UpsertSetting(context.Background(), "key2", "value2"))

	svc := setting.NewService(repo)
	err := svc.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "value1", svc.Get("key1"))
	assert.Equal(t, "value2", svc.Get("key2"))
}

func TestKeyDefaultModel(t *testing.T) {
	assert.Equal(t, "default_model", setting.KeyDefaultModel)
}
