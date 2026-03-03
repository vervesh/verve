package settingapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/setting"
	"github.com/joshjon/verve/internal/settingapi"
	"github.com/joshjon/verve/internal/sqlite"
)

type fixture struct {
	Server         *server.Server
	SettingService *setting.Service
	t              *testing.T
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := sqlite.NewTestDB(t)
	settingRepo := sqlite.NewSettingRepository(db)
	settingService := setting.NewService(settingRepo)

	handler := settingapi.NewHTTPHandler(nil, settingService, nil)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("/api/v1", handler)

	go srv.Start()
	err = srv.WaitHealthy(10, 100*time.Millisecond)
	require.NoError(t, err)

	t.Cleanup(func() { srv.Stop(context.Background()) })

	return &fixture{
		Server:         srv,
		SettingService: settingService,
		t:              t,
	}
}

func (f *fixture) defaultModelURL() string {
	return fmt.Sprintf("%s/api/v1/settings/default-model", f.Server.Address())
}

func (f *fixture) githubTokenURL() string {
	return fmt.Sprintf("%s/api/v1/settings/github-token", f.Server.Address())
}

func mustJSONReader(v any) io.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}
