package tome

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CheckpointResult reports what happened during a checkpoint operation.
type CheckpointResult struct {
	Processed int
	Skipped   int
}

// Checkpoint discovers Claude Code transcripts, deduplicates by file hash,
// parses new or changed transcripts, and records them as sessions.
func (t *Tome) Checkpoint(ctx context.Context, repoRoot string) (CheckpointResult, error) {
	transcriptDir, err := TranscriptDir(repoRoot)
	if err != nil {
		return CheckpointResult{}, err
	}

	entries, err := os.ReadDir(transcriptDir)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckpointResult{}, nil
		}
		return CheckpointResult{}, fmt.Errorf("read transcript dir: %w", err)
	}

	var result CheckpointResult

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		filePath := filepath.Join(transcriptDir, entry.Name())

		hash, err := fileHash(filePath)
		if err != nil {
			continue // skip unreadable files
		}

		// Check if already processed with same hash.
		existing, err := t.getProcessedTranscript(ctx, filePath)
		if err == nil && existing.SHA256 == hash {
			result.Skipped++
			continue
		}

		// Parse the transcript.
		f, err := os.Open(filePath) //nolint:gosec // filePath is from os.ReadDir, not user input
		if err != nil {
			continue
		}

		session, err := ParseTranscript(f, repoRoot)
		_ = f.Close()
		if err != nil {
			continue // skip unparseable transcripts
		}

		// If re-processing (same file, different hash), delete old session.
		if err == nil && existing.SessionID != "" {
			t.deleteSession(ctx, existing.SessionID)
		}

		session.TranscriptHash = hash

		if err := t.Record(ctx, session); err != nil {
			continue
		}

		if err := t.setProcessedTranscript(ctx, filePath, hash, session.ID); err != nil {
			continue
		}

		result.Processed++
	}

	return result, nil
}

// TranscriptDir returns the Claude Code transcript directory for a repo.
// Claude Code stores transcripts at ~/.claude/projects/<sanitized-repo-path>/.
func TranscriptDir(repoRoot string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	// Claude Code sanitizes the path by replacing / with - (and strips leading -)
	sanitized := sanitizeProjectPath(repoRoot)
	return filepath.Join(home, ".claude", "projects", sanitized), nil
}

// sanitizeProjectPath converts a repo path to Claude Code's project directory name.
// e.g. /Users/josh/git/projects/verve → -Users-josh-git-projects-verve
func sanitizeProjectPath(path string) string {
	return strings.ReplaceAll(path, "/", "-")
}

// fileHash computes the SHA256 hash of a file.
func fileHash(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is from os.ReadDir, not user input
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type processedTranscript struct {
	FilePath    string
	SHA256      string
	SessionID   string
	ProcessedAt int64
}

func (t *Tome) getProcessedTranscript(ctx context.Context, filePath string) (processedTranscript, error) {
	var pt processedTranscript
	err := t.db.QueryRowContext(ctx,
		"SELECT file_path, sha256, session_id, processed_at FROM processed_transcript WHERE file_path = ?",
		filePath,
	).Scan(&pt.FilePath, &pt.SHA256, &pt.SessionID, &pt.ProcessedAt)
	return pt, err
}

func (t *Tome) setProcessedTranscript(ctx context.Context, filePath, hash, sessionID string) error {
	_, err := t.db.ExecContext(ctx, `
		INSERT INTO processed_transcript (file_path, sha256, session_id, processed_at)
		VALUES (?, ?, ?, unixepoch())
		ON CONFLICT(file_path) DO UPDATE SET sha256 = ?, session_id = ?, processed_at = unixepoch()
	`, filePath, hash, sessionID, hash, sessionID)
	return err
}

func (t *Tome) deleteSession(ctx context.Context, id string) {
	_, _ = t.db.ExecContext(ctx, "DELETE FROM session WHERE id = ?", id)
}
