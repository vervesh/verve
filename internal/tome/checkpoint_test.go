package tome_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

func TestCheckpointDiscoversTranscripts(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	// Set up a fake repo root and transcript directory.
	repoRoot := t.TempDir()
	transcriptDir, err := tome.TranscriptDir(repoRoot)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(transcriptDir, 0o755))

	// Write a transcript file.
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-cp1","timestamp":"2026-03-10T14:00:00Z","gitBranch":"main","message":{"role":"user","content":"Add feature X"}}`,
		`{"type":"assistant","sessionId":"sess-cp1","message":{"role":"assistant","content":[{"type":"text","text":"Feature X requires a new handler for the /api/v1/features endpoint with validation."}]}}`,
	}, "\n")
	require.NoError(t, os.WriteFile(filepath.Join(transcriptDir, "abc123.jsonl"), []byte(transcript), 0o644))

	result, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
	assert.Equal(t, 0, result.Skipped)

	// Verify the session was recorded.
	sessions, err := tm.Log(ctx, 10, "")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "Add feature X", sessions[0].Summary)
	assert.Contains(t, sessions[0].Content, "new handler for the /api/v1/features endpoint")
	assert.NotEmpty(t, sessions[0].TranscriptHash)
}

func TestCheckpointDeduplicates(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	repoRoot := t.TempDir()
	transcriptDir, err := tome.TranscriptDir(repoRoot)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(transcriptDir, 0o755))

	transcript := `{"type":"user","sessionId":"sess-dedup","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Do something"}}`
	require.NoError(t, os.WriteFile(filepath.Join(transcriptDir, "dedup.jsonl"), []byte(transcript), 0o644))

	// First checkpoint.
	result1, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Processed)

	// Second checkpoint — same file, same hash → skip.
	result2, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.Processed)
	assert.Equal(t, 1, result2.Skipped)

	// Only one session should exist.
	sessions, err := tm.Log(ctx, 10, "")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestCheckpointReprocessesChangedFile(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	repoRoot := t.TempDir()
	transcriptDir, err := tome.TranscriptDir(repoRoot)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(transcriptDir, 0o755))

	filePath := filepath.Join(transcriptDir, "changing.jsonl")

	// Write initial transcript.
	transcript1 := `{"type":"user","sessionId":"sess-change","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Initial task"}}`
	require.NoError(t, os.WriteFile(filePath, []byte(transcript1), 0o644))

	result1, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Processed)

	// Modify the file (simulates session extension/resume).
	transcript2 := strings.Join([]string{
		`{"type":"user","sessionId":"sess-change","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Updated task"}}`,
		`{"type":"assistant","sessionId":"sess-change","message":{"role":"assistant","content":[{"type":"text","text":"Working on it now."}]}}`,
	}, "\n")
	require.NoError(t, os.WriteFile(filePath, []byte(transcript2), 0o644))

	// Re-checkpoint — different hash → re-process.
	result2, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Processed)

	// Should still have only one session (old one replaced).
	sessions, err := tm.Log(ctx, 10, "")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "Updated task", sessions[0].Summary)
}

func TestCheckpointNoTranscriptDir(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	// Use a repo root that won't have a Claude transcript directory.
	repoRoot := t.TempDir()

	result, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 0, result.Skipped)
}

func TestCheckpointSkipsNonJSONL(t *testing.T) {
	tm := openTestTome(t)
	ctx := context.Background()

	repoRoot := t.TempDir()
	transcriptDir, err := tome.TranscriptDir(repoRoot)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(transcriptDir, 0o755))

	// Write non-JSONL files.
	require.NoError(t, os.WriteFile(filepath.Join(transcriptDir, "notes.txt"), []byte("not a transcript"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(transcriptDir, "data.json"), []byte("{}"), 0o644))

	result, err := tm.Checkpoint(ctx, repoRoot)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 0, result.Skipped)
}

func TestTranscriptDir(t *testing.T) {
	dir, err := tome.TranscriptDir("/Users/josh/git/projects/verve")
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude", "projects", "-Users-josh-git-projects-verve")
	assert.Equal(t, expected, dir)
}
