package tome

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// SyncOpts configures a sync operation.
type SyncOpts struct {
	PullOnly bool   // only import from remote
	PushOnly bool   // only export to remote
	Branch   string // override branch name (default: tome/context/<user>)
}

// SyncResult reports what happened during sync.
type SyncResult struct {
	Imported int
	Exported int
}

// Sync synchronizes sessions with a git remote via orphan branches.
// Sessions are stored as JSONL on branches like tome/context/<user>.
func (t *Tome) Sync(ctx context.Context, repoDir string, user string, opts SyncOpts) (SyncResult, error) {
	var result SyncResult

	err := t.withSyncLock(func() error {
		if !opts.PushOnly {
			imported, err := t.pull(ctx, repoDir)
			if err != nil {
				return fmt.Errorf("pull: %w", err)
			}
			result.Imported = imported
		}

		if !opts.PullOnly {
			branch := opts.Branch
			if branch == "" {
				branch = "tome/context/" + sanitizeBranch(user)
			}

			exported, err := t.push(ctx, repoDir, branch)
			if err != nil {
				return fmt.Errorf("push: %w", err)
			}
			result.Exported = exported
		}

		return nil
	})

	return result, err
}

// withSyncLock acquires an exclusive file lock for sync operations.
// Since all containers share the tome data directory, this serializes
// sync across concurrent agents on the same host.
func (t *Tome) withSyncLock(fn func() error) error {
	lockPath := filepath.Join(t.dir, "sync.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

// pull fetches all tome/context* branches from the remote and imports sessions.
func (t *Tome) pull(ctx context.Context, repoDir string) (int, error) {
	// Fetch all tome branches from origin.
	_ = gitExec(ctx, repoDir, "fetch", "origin", "refs/heads/tome/context*:refs/heads/tome/context*")

	// List all local tome branches.
	out, err := gitOutput(ctx, repoDir, "for-each-ref", "--format=%(refname:short)", "refs/heads/tome/context")
	if err != nil || strings.TrimSpace(out) == "" {
		return 0, nil // no tome branches
	}

	// Collect existing session IDs to dedup.
	existingIDs, err := t.allSessionIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("load existing IDs: %w", err)
	}

	var imported int
	branches := strings.Split(strings.TrimSpace(out), "\n")
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}

		sessions, err := readBranchSessions(ctx, repoDir, branch)
		if err != nil {
			continue // skip malformed branch data
		}

		for _, s := range sessions {
			if existingIDs[s.ID] {
				continue
			}
			if err := t.importSession(ctx, s); err != nil {
				continue
			}
			existingIDs[s.ID] = true
			imported++
		}
	}

	return imported, nil
}

// push exports unexported sessions to the given branch on the remote.
func (t *Tome) push(ctx context.Context, repoDir, branch string) (int, error) {
	sessions, err := t.unexportedSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("load unexported: %w", err)
	}
	if len(sessions) == 0 {
		return 0, nil
	}

	// Fetch the latest remote state for this branch.
	_ = gitExec(ctx, repoDir, "fetch", "origin", "refs/heads/"+branch+":refs/remotes/origin/"+branch)

	// Read existing sessions from remote ref (not local).
	existing, _ := readBranchSessions(ctx, repoDir, "origin/"+branch)

	// Combine existing + new sessions into JSONL, then gzip.
	allSessions := append(existing, sessions...)
	var jsonlBuf bytes.Buffer
	if err := encodeJSONL(&jsonlBuf, allSessions); err != nil {
		return 0, fmt.Errorf("encode sessions: %w", err)
	}

	var gzBuf bytes.Buffer
	if err := gzipBytes(&gzBuf, jsonlBuf.Bytes()); err != nil {
		return 0, fmt.Errorf("compress: %w", err)
	}

	// Create blob.
	blobHash, err := gitInputOutput(ctx, repoDir, gzBuf.Bytes(), "hash-object", "-w", "--stdin")
	if err != nil {
		return 0, fmt.Errorf("hash-object: %w", err)
	}
	blobHash = strings.TrimSpace(blobHash)

	// Create tree with single file: sessions.jsonl.gz.
	treeInput := fmt.Sprintf("100644 blob %s\tsessions.jsonl.gz\n", blobHash)
	treeHash, err := gitInputOutput(ctx, repoDir, []byte(treeInput), "mktree")
	if err != nil {
		return 0, fmt.Errorf("mktree: %w", err)
	}
	treeHash = strings.TrimSpace(treeHash)

	// Create commit (with parent from remote if branch exists).
	commitMsg := fmt.Sprintf("tome: sync %d sessions", len(sessions))
	commitArgs := []string{"commit-tree", treeHash, "-m", commitMsg}

	// Use remote ref as parent to avoid divergence.
	parentHash, err := gitOutput(ctx, repoDir, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
	if err == nil {
		commitArgs = append(commitArgs, "-p", strings.TrimSpace(parentHash))
	} else {
		// Fall back to local ref if remote doesn't exist yet.
		parentHash, err = gitOutput(ctx, repoDir, "rev-parse", "--verify", "refs/heads/"+branch)
		if err == nil {
			commitArgs = append(commitArgs, "-p", strings.TrimSpace(parentHash))
		}
	}

	commitHash, err := gitInputOutput(ctx, repoDir, nil, commitArgs...)
	if err != nil {
		return 0, fmt.Errorf("commit-tree: %w", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	// Update local ref.
	if err := gitExec(ctx, repoDir, "update-ref", "refs/heads/"+branch, commitHash); err != nil {
		return 0, fmt.Errorf("update-ref: %w", err)
	}

	// Push to remote.
	if err := gitExec(ctx, repoDir, "push", "origin", branch); err != nil {
		return 0, fmt.Errorf("push: %w", err)
	}

	// Mark sessions as exported.
	if err := t.markExported(ctx, sessions); err != nil {
		return 0, fmt.Errorf("mark exported: %w", err)
	}

	return len(sessions), nil
}

// allSessionIDs returns a set of all session IDs in the database.
func (t *Tome) allSessionIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := t.db.QueryContext(ctx, "SELECT id FROM session")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// importSession inserts a session from a remote branch, marking it as already exported.
func (t *Tome) importSession(ctx context.Context, s Session) error {
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Files == nil {
		s.Files = []string{}
	}

	tagsJSON, _ := json.Marshal(s.Tags)
	filesJSON, _ := json.Marshal(s.Files)

	_, err := t.db.ExecContext(ctx, `
		INSERT INTO session (id, summary, learnings, content, tags, files, branch, status, transcript_hash, user, created_at, exported)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, s.ID, s.Summary, s.Learnings, s.Content, string(tagsJSON), string(filesJSON), s.Branch, s.Status, nullString(s.TranscriptHash), s.User, s.CreatedAt.Unix())
	return err
}

// unexportedSessions returns all sessions not yet pushed to a remote.
func (t *Tome) unexportedSessions(ctx context.Context) ([]Session, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT id, summary, learnings, content, tags, files, branch, status, transcript_hash, user, created_at
		FROM session
		WHERE exported = 0
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// markExported marks the given sessions as exported.
func (t *Tome) markExported(ctx context.Context, sessions []Session) error {
	for _, s := range sessions {
		if _, err := t.db.ExecContext(ctx, "UPDATE session SET exported = 1 WHERE id = ?", s.ID); err != nil {
			return err
		}
	}
	return nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeBranch converts a name to a safe git branch component.
// Lowercases the input and replaces non-alphanumeric runs with hyphens.
func sanitizeBranch(name string) string {
	s := strings.ToLower(name)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "default"
	}
	return s
}

// readBranchSessions reads and decodes sessions from a branch.
// Tries gzipped format first, falls back to plain JSONL for backwards compatibility.
func readBranchSessions(ctx context.Context, repoDir, branch string) ([]Session, error) {
	// Try gzipped format first.
	if data, err := gitBinaryOutput(ctx, repoDir, "show", branch+":sessions.jsonl.gz"); err == nil {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompress: %w", err)
		}
		defer r.Close()

		decompressed, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("read decompressed: %w", err)
		}
		return decodeJSONL(bytes.NewReader(decompressed))
	}

	// Fall back to uncompressed JSONL.
	content, err := gitOutput(ctx, repoDir, "show", branch+":sessions.jsonl")
	if err != nil {
		return nil, err
	}
	return decodeJSONL(strings.NewReader(content))
}

// gzipBytes compresses data with gzip.
func gzipBytes(w io.Writer, data []byte) error {
	gz := gzip.NewWriter(w)
	if _, err := gz.Write(data); err != nil {
		return err
	}
	return gz.Close()
}

// gitBinaryOutput runs a git command and returns raw stdout bytes (for binary data).
func gitBinaryOutput(ctx context.Context, repoDir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}

// gitExec runs a git command and returns an error if it fails.
func gitExec(ctx context.Context, repoDir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

// gitOutput runs a git command and returns its stdout.
func gitOutput(ctx context.Context, repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}

// gitInputOutput runs a git command with stdin data and returns stdout.
func gitInputOutput(ctx context.Context, repoDir string, input []byte, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}
