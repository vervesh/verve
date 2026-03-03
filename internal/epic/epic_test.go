package epic

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEpic(t *testing.T) {
	before := time.Now()
	e := NewEpic("repo_123", "My Epic", "Epic description")
	after := time.Now()

	assert.True(t, strings.HasPrefix(e.ID.String(), "epc_"))
	assert.Equal(t, "repo_123", e.RepoID)
	assert.Equal(t, "My Epic", e.Title)
	assert.Equal(t, "Epic description", e.Description)
	assert.Equal(t, StatusPlanning, e.Status)
	assert.NotNil(t, e.ProposedTasks)
	assert.Empty(t, e.ProposedTasks)
	assert.NotNil(t, e.TaskIDs)
	assert.Empty(t, e.TaskIDs)
	assert.NotNil(t, e.SessionLog)
	assert.Empty(t, e.SessionLog)
	assert.False(t, e.CreatedAt.Before(before))
	assert.False(t, e.CreatedAt.After(after))
	assert.Equal(t, e.CreatedAt, e.UpdatedAt)
}
