package conversationapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/conversation"
	"github.com/joshjon/verve/internal/conversationapi"
	"github.com/joshjon/verve/internal/epic"
)

// --- Create Conversation ---

func TestCreateConversation_Success(t *testing.T) {
	f := newFixture(t)

	req := conversationapi.CreateConversationRequest{
		Title: "Design Discussion",
	}
	res := testutil.Post[server.Response[conversation.Conversation]](t, f.repoConversationsURL(), req)
	assert.Equal(t, "Design Discussion", res.Data.Title)
	assert.Equal(t, conversation.StatusActive, res.Data.Status)
	assert.Equal(t, "sonnet", res.Data.Model) // default model
	assert.Empty(t, res.Data.Messages)
}

func TestCreateConversation_WithInitialMessage(t *testing.T) {
	f := newFixture(t)

	req := conversationapi.CreateConversationRequest{
		Title:          "Feature Ideas",
		InitialMessage: "I want to add dark mode",
	}
	res := testutil.Post[server.Response[conversation.Conversation]](t, f.repoConversationsURL(), req)
	assert.Equal(t, "Feature Ideas", res.Data.Title)
	assert.Len(t, res.Data.Messages, 1)
	assert.Equal(t, "user", res.Data.Messages[0].Role)
	assert.Equal(t, "I want to add dark mode", res.Data.Messages[0].Content)
	assert.NotNil(t, res.Data.PendingMessage, "should have a pending message queued")
}

func TestCreateConversation_WithModel(t *testing.T) {
	f := newFixture(t)

	req := conversationapi.CreateConversationRequest{
		Title: "Opus Conversation",
		Model: "opus",
	}
	res := testutil.Post[server.Response[conversation.Conversation]](t, f.repoConversationsURL(), req)
	assert.Equal(t, "opus", res.Data.Model)
}

func TestCreateConversation_EmptyTitle(t *testing.T) {
	f := newFixture(t)

	req := conversationapi.CreateConversationRequest{
		Title: "",
	}
	httpRes := doJSON(t, http.MethodPost, f.repoConversationsURL(), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestCreateConversation_InvalidRepoID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/repos/bad-id/conversations"
	req := conversationapi.CreateConversationRequest{
		Title: "Test",
	}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

// --- List Conversations ---

func TestListConversationsByRepo(t *testing.T) {
	f := newFixture(t)
	f.seedConversation("Conv 1")
	f.seedConversation("Conv 2")

	res := testutil.Get[server.ResponseList[conversation.Conversation]](t, f.repoConversationsURL())
	assert.Len(t, res.Data, 2)
}

func TestListConversationsByRepo_StatusFilter(t *testing.T) {
	f := newFixture(t)
	c1 := f.seedConversation("Active Conv")
	c2 := f.seedConversation("Archived Conv")
	require.NoError(t, f.ConversationStore.ArchiveConversation(context.Background(), c2.ID))

	// Default: active only
	res := testutil.Get[server.ResponseList[conversation.Conversation]](t, f.repoConversationsURL())
	assert.Len(t, res.Data, 1)
	assert.Equal(t, c1.ID.String(), res.Data[0].ID.String())

	// All statuses
	res = testutil.Get[server.ResponseList[conversation.Conversation]](t, f.repoConversationsURL()+"?status=all")
	assert.Len(t, res.Data, 2)

	// Archived only
	res = testutil.Get[server.ResponseList[conversation.Conversation]](t, f.repoConversationsURL()+"?status=archived")
	assert.Len(t, res.Data, 1)
	assert.Equal(t, c2.ID.String(), res.Data[0].ID.String())
}

// --- Get Conversation ---

func TestGetConversation_Success(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("My Conv")

	res := testutil.Get[server.Response[conversation.Conversation]](t, f.conversationURL(conv.ID))
	assert.Equal(t, "My Conv", res.Data.Title)
	assert.Equal(t, conv.ID.String(), res.Data.ID.String())
}

func TestGetConversation_NotFound(t *testing.T) {
	f := newFixture(t)

	fakeID := conversation.NewConversationID()
	httpRes := doJSON(t, http.MethodGet, f.conversationURL(fakeID), nil)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

// --- Delete Conversation ---

func TestDeleteConversation_Success(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("To Delete")

	testutil.Delete(t, f.conversationURL(conv.ID))

	// Verify deleted
	httpRes := doJSON(t, http.MethodGet, f.conversationURL(conv.ID), nil)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

func TestDeleteConversation_NotFound(t *testing.T) {
	f := newFixture(t)

	fakeID := conversation.NewConversationID()
	req, err := http.NewRequest(http.MethodDelete, f.conversationURL(fakeID), nil)
	require.NoError(t, err)
	httpRes, err := testutil.DefaultClient.Do(req)
	require.NoError(t, err)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode)
}

// --- Send Message ---

func TestSendMessage_Success(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("Chat")

	req := conversationapi.SendMessageRequest{
		Message: "What should we build?",
	}
	res := testutil.Post[server.Response[conversation.Conversation]](t, f.conversationActionURL(conv.ID, "messages"), req)
	assert.Len(t, res.Data.Messages, 1)
	assert.Equal(t, "user", res.Data.Messages[0].Role)
	assert.Equal(t, "What should we build?", res.Data.Messages[0].Content)
	assert.NotNil(t, res.Data.PendingMessage)
}

func TestSendMessage_EmptyMessage(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("Chat")

	req := conversationapi.SendMessageRequest{
		Message: "",
	}
	httpRes := doJSON(t, http.MethodPost, f.conversationActionURL(conv.ID, "messages"), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestSendMessage_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/conversations/bad-id/messages"
	req := conversationapi.SendMessageRequest{
		Message: "hello",
	}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

// --- Archive Conversation ---

func TestArchiveConversation_Success(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("To Archive")

	res := testutil.Post[server.Response[conversation.Conversation]](t, f.conversationActionURL(conv.ID, "archive"), nil)
	assert.Equal(t, conversation.StatusArchived, res.Data.Status)

	// Verify from DB
	stored, err := f.ConversationStore.ReadConversation(context.Background(), conv.ID)
	require.NoError(t, err)
	assert.Equal(t, conversation.StatusArchived, stored.Status)
}

// --- Generate Tasks ---

func TestGenerateTasks_Success(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversationWithMessages("Planning Chat")

	req := conversationapi.GenerateTasksRequest{
		Title:          "Build REST API",
		PlanningPrompt: "Focus on security aspects",
	}
	res := testutil.Post[server.Response[epic.Epic]](t, f.conversationActionURL(conv.ID, "generate-tasks"), req)
	assert.Equal(t, "Build REST API", res.Data.Title)
	assert.Contains(t, res.Data.Description, "Tasks generated from conversation: Planning Chat")
	assert.Equal(t, epic.StatusPlanning, res.Data.Status)
	assert.Equal(t, "sonnet", res.Data.Model)

	// Verify planning prompt contains conversation transcript
	assert.Contains(t, res.Data.PlanningPrompt, "--- Conversation Transcript ---")
	assert.Contains(t, res.Data.PlanningPrompt, "User: Hello, what should we build?")
	assert.Contains(t, res.Data.PlanningPrompt, "Assistant: I suggest we build a REST API.")
	assert.Contains(t, res.Data.PlanningPrompt, "User: Great idea, let's add auth too.")
	assert.Contains(t, res.Data.PlanningPrompt, "Assistant: We can use JWT-based authentication.")
	assert.Contains(t, res.Data.PlanningPrompt, "--- End Transcript ---")
	assert.Contains(t, res.Data.PlanningPrompt, "Focus on security aspects")

	// Verify conversation now has epic_id linked
	stored, err := f.ConversationStore.ReadConversation(context.Background(), conv.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.EpicID)
	assert.Equal(t, res.Data.ID.String(), *stored.EpicID)
}

func TestGenerateTasks_Conflict_EpicAlreadyLinked(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversationWithMessages("Chat")

	// First generate-tasks should succeed
	req := conversationapi.GenerateTasksRequest{
		Title: "First Epic",
	}
	testutil.Post[server.Response[epic.Epic]](t, f.conversationActionURL(conv.ID, "generate-tasks"), req)

	// Second generate-tasks should fail with 409
	req2 := conversationapi.GenerateTasksRequest{
		Title: "Second Epic",
	}
	httpRes := doJSON(t, http.MethodPost, f.conversationActionURL(conv.ID, "generate-tasks"), req2)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusConflict, httpRes.StatusCode)
}

func TestGenerateTasks_ArchivedConversation(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("Archived")
	require.NoError(t, f.ConversationStore.ArchiveConversation(context.Background(), conv.ID))

	req := conversationapi.GenerateTasksRequest{
		Title: "Tasks",
	}
	httpRes := doJSON(t, http.MethodPost, f.conversationActionURL(conv.ID, "generate-tasks"), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusConflict, httpRes.StatusCode)
}

func TestGenerateTasks_EmptyTitle(t *testing.T) {
	f := newFixture(t)
	conv := f.seedConversation("Chat")

	req := conversationapi.GenerateTasksRequest{
		Title: "",
	}
	httpRes := doJSON(t, http.MethodPost, f.conversationActionURL(conv.ID, "generate-tasks"), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}
