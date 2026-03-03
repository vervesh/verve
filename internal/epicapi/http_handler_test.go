package epicapi_test

import (
	"net/http"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/epicapi"
)

func TestCreateEpic_Success(t *testing.T) {
	f := newFixture(t)

	req := epicapi.CreateEpicRequest{
		Title:       "My Epic",
		Description: "Epic description",
	}
	res := testutil.Post[server.Response[epic.Epic]](t, f.repoEpicsURL(), req)
	assert.Equal(t, "My Epic", res.Data.Title)
	assert.Equal(t, "Epic description", res.Data.Description)
	assert.Equal(t, epic.StatusPlanning, res.Data.Status)
	assert.Equal(t, "sonnet", res.Data.Model) // default model
}

func TestCreateEpic_EmptyTitle(t *testing.T) {
	f := newFixture(t)

	req := epicapi.CreateEpicRequest{
		Title:       "",
		Description: "desc",
	}
	httpRes := doJSON(t, http.MethodPost, f.repoEpicsURL(), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestCreateEpic_InvalidRepoID(t *testing.T) {
	f := newFixture(t)

	// Use a URL with an invalid repo ID
	url := f.Server.Address() + "/api/v1/repos/bad-id/epics"
	req := epicapi.CreateEpicRequest{
		Title:       "Epic",
		Description: "desc",
	}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestListEpicsByRepo(t *testing.T) {
	f := newFixture(t)
	f.seedEpic("Epic 1", "desc 1")

	res := testutil.Get[server.ResponseList[epic.Epic]](t, f.repoEpicsURL())
	assert.Len(t, res.Data, 1)
	assert.Equal(t, "Epic 1", res.Data[0].Title)
}

func TestGetEpic_Success(t *testing.T) {
	f := newFixture(t)
	e := f.seedEpic("My Epic", "desc")

	res := testutil.Get[server.Response[epic.Epic]](t, f.epicURL(e.ID))
	assert.Equal(t, "My Epic", res.Data.Title)
	assert.Equal(t, e.ID.String(), res.Data.ID.String())
}

func TestGetEpic_NotFound(t *testing.T) {
	f := newFixture(t)

	fakeID := epic.NewEpicID()
	httpRes := doJSON(t, http.MethodGet, f.epicURL(fakeID), nil)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

func TestDeleteEpic_Success(t *testing.T) {
	f := newFixture(t)
	e := f.seedEpic("To Delete", "desc")

	testutil.Delete(t, f.epicURL(e.ID))

	// Verify deleted
	httpRes := doJSON(t, http.MethodGet, f.epicURL(e.ID), nil)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

func TestDeleteEpic_NotFound(t *testing.T) {
	f := newFixture(t)

	fakeID := epic.NewEpicID()
	req, err := http.NewRequest(http.MethodDelete, f.epicURL(fakeID), nil)
	require.NoError(t, err)
	httpRes, err := testutil.DefaultClient.Do(req)
	require.NoError(t, err)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

func TestStartPlanning_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/epics/bad-id/plan"
	req := map[string]string{"prompt": "plan this"}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestUpdateProposedTasks(t *testing.T) {
	f := newFixture(t)
	e := f.seedDraftEpic("Draft Epic", "desc")

	req := epicapi.UpdateProposedTasksRequest{
		Tasks: []epic.ProposedTask{
			{TempID: "t1", Title: "Task 1", Description: "desc 1"},
			{TempID: "t2", Title: "Task 2", Description: "desc 2"},
		},
	}
	res := testutil.Put[server.Response[epic.Epic]](t, f.epicActionURL(e.ID, "proposed-tasks"), req)
	assert.Len(t, res.Data.ProposedTasks, 2)
	assert.Equal(t, epic.StatusDraft, res.Data.Status)
}

func TestSendSessionMessage(t *testing.T) {
	f := newFixture(t)
	e := f.seedDraftEpic("Epic", "desc")

	req := epicapi.SessionMessageRequest{
		Message: "Please add error handling",
	}
	res := testutil.Post[server.Response[epic.Epic]](t, f.epicActionURL(e.ID, "session-message"), req)
	assert.Contains(t, res.Data.SessionLog, "user: Please add error handling")
	// Should transition back to planning for re-queue
	assert.Equal(t, epic.StatusPlanning, res.Data.Status)
}

func TestConfirmEpic(t *testing.T) {
	f := newFixture(t)
	e := f.seedDraftEpic("Epic", "desc")

	req := epicapi.ConfirmEpicRequest{}
	res := testutil.Post[server.Response[epic.Epic]](t, f.epicActionURL(e.ID, "confirm"), req)
	assert.Equal(t, epic.StatusActive, res.Data.Status)
	assert.Len(t, res.Data.TaskIDs, 1)
}

func TestCloseEpic(t *testing.T) {
	f := newFixture(t)
	e := f.seedEpic("Epic", "desc")

	res := testutil.Post[server.Response[epic.Epic]](t, f.epicActionURL(e.ID, "close"), nil)
	assert.Equal(t, epic.StatusClosed, res.Data.Status)
}

func TestGetEpicTasks(t *testing.T) {
	f := newFixture(t)
	e := f.seedDraftEpic("Epic", "desc")

	// Confirm to create real tasks
	req := epicapi.ConfirmEpicRequest{}
	testutil.Post[server.Response[epic.Epic]](t, f.epicActionURL(e.ID, "confirm"), req)

	res := testutil.Get[server.ResponseList[epicapi.EpicTaskSummary]](t, f.epicActionURL(e.ID, "tasks"))
	assert.Len(t, res.Data, 1)
	assert.Equal(t, "Sub-task 1", res.Data[0].Title)
}
