package tome_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

func TestParseTranscriptBasic(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-123","timestamp":"2026-03-10T14:00:00Z","gitBranch":"feat/auth","message":{"role":"user","content":"Add authentication middleware"}}`,
		`{"type":"assistant","sessionId":"sess-123","timestamp":"2026-03-10T14:01:00Z","message":{"role":"assistant","content":[{"type":"text","text":"I'll add authentication middleware to the project."},{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/auth.go"}},{"type":"tool_result","content":"file contents here"},{"type":"text","text":"The auth module needs a bearer token validator."},{"type":"tool_use","name":"Write","input":{"file_path":"/repo/src/middleware.go"}}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "sess-123", session.ID)
	assert.Equal(t, "Add authentication middleware", session.Summary)
	assert.Equal(t, "feat/auth", session.Branch)
	assert.Contains(t, session.Content, "The auth module needs a bearer token validator.")
	// Write files come first, then read-only files.
	assert.Contains(t, session.Files, "src/middleware.go")
	assert.Contains(t, session.Files, "src/auth.go")
	assert.Equal(t, "succeeded", session.Status)
}

func TestParseTranscriptSkipsSidechain(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-456","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Fix the bug"}}`,
		`{"type":"assistant","sessionId":"sess-456","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"Sidechain exploration text"}]}}`,
		`{"type":"assistant","sessionId":"sess-456","message":{"role":"assistant","content":[{"type":"text","text":"Main branch fix"}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.NotContains(t, session.Content, "Sidechain exploration")
	assert.Contains(t, session.Content, "Main branch fix")
}

func TestParseTranscriptSkipsThinking(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-789","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Refactor auth"}}`,
		`{"type":"assistant","sessionId":"sess-789","message":{"role":"assistant","content":[{"type":"thinking","text":"Let me think about this..."},{"type":"text","text":"The auth module needs to be refactored to use OAuth2 flows."}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.NotContains(t, session.Content, "Let me think")
	assert.Contains(t, session.Content, "auth module needs to be refactored")
}

func TestParseTranscriptSkipsFileHistorySnapshot(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-abc","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Update config"}}`,
		`{"type":"file-history-snapshot","sessionId":"sess-abc"}`,
		`{"type":"assistant","sessionId":"sess-abc","message":{"role":"assistant","content":[{"type":"text","text":"Updated the config."}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "sess-abc", session.ID)
	assert.Contains(t, session.Content, "Updated the config.")
}

func TestParseTranscriptSummaryTruncation(t *testing.T) {
	longMessage := strings.Repeat("x", 300)
	transcript := `{"type":"user","sessionId":"sess-long","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"` + longMessage + `"}}`

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Len(t, session.Summary, 200)
}

func TestParseTranscriptSummaryFirstLine(t *testing.T) {
	transcript := `{"type":"user","sessionId":"sess-multi","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"First line of task\nSecond line with details\nThird line"}}`

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "First line of task", session.Summary)
}

func TestParseTranscriptDeduplicatesFiles(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-dedup","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Edit files"}}`,
		`{"type":"assistant","sessionId":"sess-dedup","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/main.go"}},{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/src/main.go"}},{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/util.go"}}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	// Should have exactly 2 unique files, not 3.
	assert.Len(t, session.Files, 2)
	assert.Contains(t, session.Files, "src/main.go")
	assert.Contains(t, session.Files, "src/util.go")
}

func TestParseTranscriptNoSessionID(t *testing.T) {
	transcript := `{"type":"user","message":{"role":"user","content":"Hello"}}`

	_, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session ID")
}

func TestParseTranscriptStringContent(t *testing.T) {
	// User messages can have string content (not array of blocks).
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-str","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Simple string content that is a real task description"}}`,
		`{"type":"assistant","sessionId":"sess-str","message":{"role":"assistant","content":"Simple assistant response"}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "Simple string content that is a real task description", session.Summary)
	assert.Contains(t, session.Content, "Simple assistant response")
}

func TestParseTranscriptSkipsInterruptedSummary(t *testing.T) {
	// First user message is a tool continuation — should skip it for summary.
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-int","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"[Request interrupted by user for tool use]"}}`,
		`{"type":"assistant","sessionId":"sess-int","message":{"role":"assistant","content":[{"type":"text","text":"The authentication module needs updating to support OAuth2 flows."}]}}`,
		`{"type":"user","sessionId":"sess-int","timestamp":"2026-03-10T14:02:00Z","message":{"role":"user","content":"Add OAuth2 support to the auth module"}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "Add OAuth2 support to the auth module", session.Summary)
}

func TestParseTranscriptFallsBackToAssistantContent(t *testing.T) {
	// All user messages are non-substantive — should fall back to assistant content.
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-fb","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"[Request interrupted by user for tool use]"}}`,
		`{"type":"assistant","sessionId":"sess-fb","message":{"role":"assistant","content":[{"type":"text","text":"I'll start by reading the files."},{"type":"text","text":"The error handling in the API layer needs to return proper HTTP status codes instead of 500 for all errors."}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	// Should skip "I'll start by..." preamble and use the substantive line.
	assert.Equal(t, "The error handling in the API layer needs to return proper HTTP status codes instead of 500 for all errors.", session.Summary)
}

func TestParseTranscriptSkipsShortAcks(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-ack","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"ok"}}`,
		`{"type":"user","sessionId":"sess-ack","timestamp":"2026-03-10T14:01:00Z","message":{"role":"user","content":"yes"}}`,
		`{"type":"user","sessionId":"sess-ack","timestamp":"2026-03-10T14:02:00Z","message":{"role":"user","content":"Implement the user settings page with dark mode toggle"}}`,
		`{"type":"assistant","sessionId":"sess-ack","message":{"role":"assistant","content":[{"type":"text","text":"Working on it."}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "Implement the user settings page with dark mode toggle", session.Summary)
}

func TestParseTranscriptSummarySkipsSyntheticPrefix(t *testing.T) {
	transcript := `{"type":"user","sessionId":"sess-prefix","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Implement the following plan:\n\n# Tome — Auto-Capture from Claude Code Transcripts\n\nMore details here."}}`

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	// Should skip the synthetic "Implement the following plan:" and use the heading.
	assert.Equal(t, "Tome — Auto-Capture from Claude Code Transcripts", session.Summary)
}

func TestParseTranscriptSummaryPreservesRealColonMessages(t *testing.T) {
	// A real user message ending with ":" should NOT be skipped.
	transcript := `{"type":"user","sessionId":"sess-colon","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"The issue is with the auth flow, for example:\nthe token refresh fails silently"}}`

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "The issue is with the auth flow, for example:", session.Summary)
}

func TestParseTranscriptWriteFilesPrioritized(t *testing.T) {
	// Write files should appear before read-only files.
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-pri","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Refactor the API handlers"}}`,
		`{"type":"assistant","sessionId":"sess-pri","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/z_readonly.go"}},{"type":"tool_use","name":"Write","input":{"file_path":"/repo/a_written.go"}}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	require.Len(t, session.Files, 2)
	// Written file first (sorted), then read-only.
	assert.Equal(t, "a_written.go", session.Files[0])
	assert.Equal(t, "z_readonly.go", session.Files[1])
}

func TestParseTranscriptFilesCapped(t *testing.T) {
	// Generate a transcript with many files.
	var toolUses []string
	for i := 0; i < 30; i++ {
		toolUses = append(toolUses, fmt.Sprintf(`{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/file%02d.go"}}`, i))
	}

	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-cap","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Refactor all the things"}}`,
		`{"type":"assistant","sessionId":"sess-cap","message":{"role":"assistant","content":[` + strings.Join(toolUses, ",") + `]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.LessOrEqual(t, len(session.Files), 15)
}
