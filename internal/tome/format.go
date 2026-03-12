package tome

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// FormatSearchResults writes search results in human-readable text format.
func FormatSearchResults(w io.Writer, results []SearchResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No sessions found.")
		return
	}

	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		formatSession(w, r.Session)
	}
}

// FormatLog writes sessions in human-readable text format.
func FormatLog(w io.Writer, sessions []Session) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "No sessions recorded.")
		return
	}

	for i, s := range sessions {
		if i > 0 {
			fmt.Fprintln(w)
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

func formatSession(w io.Writer, s Session) {
	source := "manual"
	if s.TranscriptHash != "" {
		source = "transcript"
	}
	fmt.Fprintf(w, "━━ %s (%s, %s) ━━\n", s.Summary, s.Status, relativeTime(s.CreatedAt))

	if len(s.Files) > 0 {
		if len(s.Files) > 10 {
			fmt.Fprintf(w, "Files: %s (+%d more)\n", strings.Join(s.Files[:10], ", "), len(s.Files)-10)
		} else {
			fmt.Fprintf(w, "Files: %s\n", strings.Join(s.Files, ", "))
		}
	}
	if len(s.Tags) > 0 {
		fmt.Fprintf(w, "Tags:  %s\n", strings.Join(s.Tags, ", "))
	}
	if s.Branch != "" {
		fmt.Fprintf(w, "Branch: %s\n", s.Branch)
	}
	if s.User != "" {
		fmt.Fprintf(w, "User: %s\n", s.User)
	}
	fmt.Fprintf(w, "Source: %s\n", source)
	if s.Learnings != "" {
		fmt.Fprintln(w, "Learnings:")
		for _, line := range strings.Split(s.Learnings, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
	if s.Content != "" {
		if preview := contentPreview(s.Content); preview != "" {
			fmt.Fprintf(w, "Content: %s\n", preview)
		}
	}
}

// contentPreview returns the last substantive line from content.
// The end of a conversation has conclusions and results; the beginning has
// filler ("Let me read the files..."). We scan backwards to find it.
func contentPreview(content string) string {
	lines := strings.Split(content, "\n")

	// Walk backwards to find the last substantive line.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if isPreamble(line) {
			continue
		}
		// Skip very short lines (often "Done.", "All passing.", etc.)
		if len(line) < 20 {
			continue
		}
		if len(line) > 200 {
			line = line[:200] + "..."
		}
		return line
	}
	return ""
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
