package tome

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// DetectRepo returns the normalized "owner/name" for the repository at repoDir.
// Checks GITHUB_REPO env var first (set in agent containers), then falls back
// to parsing the origin remote URL.
func DetectRepo(ctx context.Context, repoDir string) string {
	if r := os.Getenv("GITHUB_REPO"); r != "" {
		return NormalizeRepo(r)
	}

	out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return NormalizeRepo(strings.TrimSpace(string(out)))
}

// NormalizeRepo extracts "owner/name" from a git remote URL or plain "owner/name" string.
// Handles HTTPS URLs, SSH URLs, and plain owner/name. Lowercased, .git suffix stripped.
// Returns empty string for invalid input.
func NormalizeRepo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var ownerName string

	switch {
	case strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://"):
		// https://github.com/owner/name.git → owner/name
		raw = strings.TrimPrefix(raw, "https://")
		raw = strings.TrimPrefix(raw, "http://")
		parts := strings.SplitN(raw, "/", 3)
		if len(parts) < 3 {
			return ""
		}
		ownerName = parts[1] + "/" + parts[2]

	case strings.Contains(raw, ":") && strings.Contains(raw, "@"):
		// git@github.com:owner/name.git → owner/name
		idx := strings.Index(raw, ":")
		ownerName = raw[idx+1:]

	default:
		// Plain owner/name.
		ownerName = raw
	}

	// Strip .git suffix.
	ownerName = strings.TrimSuffix(ownerName, ".git")

	// Validate it looks like owner/name.
	parts := strings.Split(ownerName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}

	return strings.ToLower(ownerName)
}
