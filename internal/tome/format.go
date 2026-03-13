package tome

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// FormatSearchResults writes search results in human-readable text format.
// The query is used to extract a context snippet from session content.
func FormatSearchResults(w io.Writer, results []SearchResult, query string) {
	if len(results) == 0 {
		_, _ = fmt.Fprintln(w, "No sessions found.")
		return
	}

	for i, r := range results {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		formatSession(w, r.Session)
		if snippet := matchSnippet(r.Session.Content, query, 500); snippet != "" {
			_, _ = fmt.Fprintf(w, "Match: ...%s...\n", snippet)
		}
	}
}

// FormatLog writes sessions in human-readable text format.
func FormatLog(w io.Writer, sessions []Session) {
	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(w, "No sessions recorded.")
		return
	}

	for i, s := range sessions {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		formatSession(w, s)
	}
}

// FormatJSON writes the value as indented JSON.
func FormatJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// FormatSessionDetail writes a single session with its full content.
func FormatSessionDetail(w io.Writer, s Session) {
	formatSession(w, s)
	if s.Content != "" {
		_, _ = fmt.Fprintln(w, "Content:")
		_, _ = fmt.Fprintln(w, s.Content)
	}
}

func formatSession(w io.Writer, s Session) {
	source := "manual"
	if s.TranscriptHash != "" {
		source = "transcript"
	}
	_, _ = fmt.Fprintf(w, "━━ %s (%s, %s) ━━\n", s.Summary, s.Status, relativeTime(s.CreatedAt))
	_, _ = fmt.Fprintf(w, "ID: %s\n", s.ID)

	if len(s.Files) > 0 {
		if len(s.Files) > 10 {
			_, _ = fmt.Fprintf(w, "Files: %s (+%d more)\n", strings.Join(s.Files[:10], ", "), len(s.Files)-10)
		} else {
			_, _ = fmt.Fprintf(w, "Files: %s\n", strings.Join(s.Files, ", "))
		}
	}
	if len(s.Tags) > 0 {
		_, _ = fmt.Fprintf(w, "Tags:  %s\n", strings.Join(s.Tags, ", "))
	}
	if s.Repo != "" {
		_, _ = fmt.Fprintf(w, "Repo: %s\n", s.Repo)
	}
	if s.Branch != "" {
		_, _ = fmt.Fprintf(w, "Branch: %s\n", s.Branch)
	}
	if s.User != "" {
		_, _ = fmt.Fprintf(w, "User: %s\n", s.User)
	}
	_, _ = fmt.Fprintf(w, "Source: %s\n", source)
	if s.Learnings != "" {
		_, _ = fmt.Fprintln(w, "Learnings:")
		for _, line := range strings.Split(s.Learnings, "\n") {
			_, _ = fmt.Fprintf(w, "  %s\n", line)
		}
	}
}

// matchSnippet extracts a context window from content centered on the first
// occurrence of any query term. Returns empty string if no match found.
func matchSnippet(content, query string, windowSize int) string {
	if content == "" || query == "" {
		return ""
	}

	contentLower := strings.ToLower(content)
	terms := strings.Fields(strings.ToLower(query))

	// Find the earliest match position across all query terms.
	bestPos := -1
	for _, term := range terms {
		if pos := strings.Index(contentLower, term); pos >= 0 {
			if bestPos < 0 || pos < bestPos {
				bestPos = pos
			}
		}
	}

	if bestPos < 0 {
		return ""
	}

	// Center the window around the match.
	half := windowSize / 2
	start := bestPos - half
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end > len(content) {
		end = len(content)
		start = end - windowSize
		if start < 0 {
			start = 0
		}
	}

	snippet := content[start:end]

	// Clean up: collapse whitespace and trim to word boundaries.
	snippet = strings.Join(strings.Fields(snippet), " ")

	return snippet
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
