package tome

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const tomeMarker = "# managed by tome"

const claudeMDMarkerStart = "<!-- tome:start -->"
const claudeMDMarkerEnd = "<!-- tome:end -->"

const claudeMDSection = `<!-- tome:start -->
## Session Memory (Tome)

Use the /tome skill to manage session memory:
- **Before starting work**: Search for relevant prior sessions to find useful context
- **After completing work**: Record a structured summary of what was done, key decisions, and learnings

This helps maintain continuity across sessions so future work can build on past context.
<!-- tome:end -->`

const postCommitHook = `#!/bin/sh
# managed by tome
if command -v tome >/dev/null 2>&1; then
  tome checkpoint 2>/dev/null &
fi
`

const prePushHook = `#!/bin/sh
# managed by tome
if command -v tome >/dev/null 2>&1; then
  tome sync --push 2>/dev/null || true
fi
`

const skillContent = `---
name: tome
description: |
  Use this skill to search or record session memory with tome.
  Invoke when: starting a new task (search for context), finishing a task (record what was done),
  the user asks about past sessions, or you need context about prior work on a file or feature.
allowed-tools: Bash(tome *)
argument-hint: [search query or "record"]
---

# Tome — Session Memory

Tome is a local session memory ledger. Use it to find context from prior sessions and to record
what you learned and decided during the current session.

## Commands

### Search for context

Before starting work, search for relevant prior sessions:

` + "```bash" + `
tome search "authentication middleware"    # semantic + keyword search
tome search --file "src/auth" "token"      # filter by files touched
tome search -n 10 "database migration"     # more results
` + "```" + `

Search returns sessions with a match snippet. To drill into a specific session:

` + "```bash" + `
tome show <session-id>                     # full session content
tome show <session-id> --json              # structured output
` + "```" + `

### Record a session

After completing work, record a structured summary of what was done:

` + "```bash" + `
tome record \
  --summary "Added OAuth2 middleware with token refresh" \
  --learnings "The auth module uses a middleware chain pattern. Token refresh needs to happen before route handlers, not after. The existing bearer validator in auth.go can be extended rather than replaced." \
  --files "src/auth/middleware.go,src/auth/oauth2.go,src/routes/api.go" \
  --tags "auth,oauth2,middleware" \
  --status succeeded
` + "```" + `

### View recent sessions

` + "```bash" + `
tome log                  # last 10 sessions
tome log -n 20            # last 20
tome log --json           # structured output
` + "```" + `

## What to Record

When recording, focus on information that would help a future developer (human or AI) working on
the same code. Write the --learnings field as if briefing a colleague.

**Include:**
- Key decisions and *why* they were made (not just what)
- Problems encountered and how they were solved
- Non-obvious patterns or conventions discovered in the codebase
- Gotchas, constraints, or things that almost went wrong
- Architecture insights ("X depends on Y because...")

**Exclude:**
- Step-by-step narration of what you did ("First I read the file, then I...")
- Tool outputs or file contents
- Obvious facts that anyone reading the code would see

**Example learnings:**

The task store uses a broker pattern for real-time events. Mutations in store.go
publish to an in-memory broker, which fans out to SSE subscribers. When adding new
mutation methods, always call s.broker.Publish() after the DB write or SSE clients
won't update. The PG notifier in postgres/notifier.go handles cross-instance fan-out
for horizontal scaling, but the SQLite path uses local-only fan-out (nil notifier).

## When to Use

| Situation | Action |
|-----------|--------|
| Starting a new task | ` + "`tome search \"relevant topic\"`" + ` to find prior context |
| Modifying unfamiliar code | ` + "`tome search --file \"path/to/file\"`" + ` to see who changed it and why |
| Finished a task | ` + "`tome record --summary \"...\" --learnings \"...\"`" + ` |
| User asks about past work | ` + "`tome search`" + ` or ` + "`tome log`" + ` |
| Debugging a regression | ` + "`tome search --file \"affected/file\" \"feature name\"`" + ` |
`

// InstallSkill installs the tome Claude Code skill at .claude/skills/tome/SKILL.md.
// Idempotent — overwrites existing skill file with latest content.
func InstallSkill(repoDir string) error {
	skillDir := filepath.Join(repoDir, ".claude", "skills", "tome")
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o600)
}

// RemoveSkill removes the tome Claude Code skill directory.
func RemoveSkill(repoDir string) error {
	skillDir := filepath.Join(repoDir, ".claude", "skills", "tome")
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("remove skill directory: %w", err)
	}

	// Clean up empty parent dirs (.claude/skills, .claude) if we left them empty.
	for _, dir := range []string{
		filepath.Join(repoDir, ".claude", "skills"),
		filepath.Join(repoDir, ".claude"),
	} {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}
	return nil
}

// AddClaudeMD appends a tome instructions section to the repo's CLAUDE.md.
// Idempotent — skips if the marker is already present.
func AddClaudeMD(repoDir string) error {
	claudeMDPath := filepath.Join(repoDir, "CLAUDE.md")

	existing, err := os.ReadFile(claudeMDPath) //nolint:gosec // path is constructed from repoDir, not user input
	if err == nil {
		if strings.Contains(string(existing), claudeMDMarkerStart) {
			return nil // already present
		}

		// Append to existing CLAUDE.md.
		content := string(existing)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + claudeMDSection + "\n"
		return os.WriteFile(claudeMDPath, []byte(content), 0o600)
	}

	// No CLAUDE.md — create one.
	return os.WriteFile(claudeMDPath, []byte(claudeMDSection+"\n"), 0o600)
}

// RemoveClaudeMD removes the tome section from CLAUDE.md.
// Removes the file entirely if the tome section was the only content.
func RemoveClaudeMD(repoDir string) error {
	claudeMDPath := filepath.Join(repoDir, "CLAUDE.md")

	existing, readErr := os.ReadFile(claudeMDPath)
	if readErr != nil {
		return nil // no CLAUDE.md
	}

	content := string(existing)
	if !strings.Contains(content, claudeMDMarkerStart) {
		return nil // no tome section
	}

	// Remove everything from start marker to end marker (inclusive).
	startIdx := strings.Index(content, claudeMDMarkerStart)
	endIdx := strings.Index(content, claudeMDMarkerEnd)
	if startIdx == -1 || endIdx == -1 {
		return nil
	}
	endIdx += len(claudeMDMarkerEnd)

	// Remove the section plus surrounding blank lines.
	before := strings.TrimRight(content[:startIdx], "\n")
	after := strings.TrimLeft(content[endIdx:], "\n")

	var result string
	if before != "" && after != "" {
		result = before + "\n\n" + after
	} else {
		result = before + after
	}

	if strings.TrimSpace(result) == "" {
		return os.Remove(claudeMDPath)
	}

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return os.WriteFile(claudeMDPath, []byte(result), 0o644)
}

// InstallHooks installs post-commit and pre-push git hooks for automatic
// transcript capture and sync. Idempotent — skips if marker already present.
// Preserves existing hook content by appending.
func InstallHooks(repoDir string) error {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}

	if err := installHook(hooksDir, "post-commit", postCommitHook); err != nil {
		return fmt.Errorf("install post-commit hook: %w", err)
	}
	if err := installHook(hooksDir, "pre-push", prePushHook); err != nil {
		return fmt.Errorf("install pre-push hook: %w", err)
	}
	return nil
}

// AddGitignore ensures .tome is listed in the repo's .gitignore.
// Idempotent — skips if already present.
func AddGitignore(repoDir string) error {
	gitignorePath := filepath.Join(repoDir, ".gitignore")

	existing, err := os.ReadFile(gitignorePath)
	if err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == ".tome" || strings.TrimSpace(line) == ".tome/" {
				return nil // already present
			}
		}

		// Append .tome to existing .gitignore.
		content := string(existing)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += ".tome/\n"
		return os.WriteFile(gitignorePath, []byte(content), 0o644)
	}

	// No .gitignore — create one.
	return os.WriteFile(gitignorePath, []byte(".tome/\n"), 0o644)
}

// UninstallHooks removes tome-managed sections from git hooks.
// If the hook file only contains tome content, the file is removed entirely.
func UninstallHooks(repoDir string) error {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil // no hooks dir, nothing to do
	}

	for _, name := range []string{"post-commit", "pre-push"} {
		if err := uninstallHook(hooksDir, name); err != nil {
			return fmt.Errorf("uninstall %s hook: %w", name, err)
		}
	}
	return nil
}

// RemoveGitignore removes the .tome/ entry from .gitignore.
// Removes the file entirely if .tome/ was the only entry.
func RemoveGitignore(repoDir string) error {
	gitignorePath := filepath.Join(repoDir, ".gitignore")

	existing, readErr := os.ReadFile(gitignorePath)
	if readErr != nil {
		return nil // no .gitignore, nothing to do
	}

	lines := strings.Split(string(existing), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".tome" || trimmed == ".tome/" {
			continue
		}
		filtered = append(filtered, line)
	}

	// If only empty lines remain, remove the file.
	content := strings.Join(filtered, "\n")
	if strings.TrimSpace(content) == "" {
		return os.Remove(gitignorePath)
	}

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}

func uninstallHook(hooksDir, name string) error {
	hookPath := filepath.Join(hooksDir, name)

	existing, readErr := os.ReadFile(hookPath)
	if readErr != nil {
		return nil // hook doesn't exist
	}

	content := string(existing)
	if !strings.Contains(content, tomeMarker) {
		return nil // no tome content
	}

	// Remove the tome-managed block. Split into lines and remove everything
	// from the marker line through the end of the tome block.
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	inTomeBlock := false
	for _, line := range lines {
		if strings.Contains(line, tomeMarker) {
			inTomeBlock = true
			continue
		}
		if inTomeBlock {
			// Tome blocks end at the next empty line or shebang.
			// The block includes: marker, if/fi block, trailing empty lines.
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue // skip blank lines within/after tome block
			}
			if strings.HasPrefix(trimmed, "#!") || (!strings.HasPrefix(trimmed, "if ") && !strings.HasPrefix(trimmed, "tome ") && trimmed != "fi") {
				// Non-tome content — stop removing.
				inTomeBlock = false
				filtered = append(filtered, line)
			}
			// else: still in tome block, skip line
			continue
		}
		filtered = append(filtered, line)
	}

	remaining := strings.Join(filtered, "\n")
	remaining = strings.TrimRight(remaining, "\n")

	// If only a shebang (or nothing) remains, remove the file entirely.
	trimmed := strings.TrimSpace(remaining)
	if trimmed == "" || trimmed == "#!/bin/sh" || trimmed == "#!/bin/bash" {
		return os.Remove(hookPath)
	}

	return os.WriteFile(hookPath, []byte(remaining+"\n"), 0o755)
}

func installHook(hooksDir, name, content string) error {
	hookPath := filepath.Join(hooksDir, name)

	existing, err := os.ReadFile(hookPath)
	if err == nil {
		// Hook file exists — check for marker.
		if strings.Contains(string(existing), tomeMarker) {
			return nil // already installed
		}

		// Append tome hook to existing content.
		combined := string(existing)
		if !strings.HasSuffix(combined, "\n") {
			combined += "\n"
		}
		combined += "\n" + content
		return os.WriteFile(hookPath, []byte(combined), 0o755)
	}

	// No existing hook — create new.
	return os.WriteFile(hookPath, []byte(content), 0o755)
}
