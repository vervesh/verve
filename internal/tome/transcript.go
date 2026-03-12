package tome

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type transcriptEntry struct {
	Type        string             `json:"type"`
	SessionID   string             `json:"sessionId"`
	Timestamp   string             `json:"timestamp"`
	GitBranch   string             `json:"gitBranch"`
	IsSidechain bool               `json:"isSidechain"`
	Message     *transcriptMessage `json:"message"`
}

type transcriptMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type  string          `json:"type"`  // "text", "tool_use", "tool_result", "thinking"
	Text  string          `json:"text"`
	Name  string          `json:"name"`  // tool name for tool_use
	Input json.RawMessage `json:"input"` // tool parameters
}

// toolInput is used to extract file_path from tool_use inputs.
type toolInput struct {
	FilePath string `json:"file_path"`
}

// writeToolNames tracks tools that modify files (stronger signal than reads).
var writeToolNames = map[string]bool{
	"Write":        true,
	"Edit":         true,
	"NotebookEdit": true,
}

// maxFiles caps the number of files stored per session.
const maxFiles = 15

// ParseTranscript reads a Claude Code .jsonl transcript and extracts a Session.
// The repoRoot is used to strip absolute paths to relative paths.
func ParseTranscript(r io.Reader, repoRoot string) (Session, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB buffer for large tool results

	var (
		sessionID    string
		branch       string
		summary      string
		createdAt    time.Time
		content      strings.Builder
		writeFiles   = make(map[string]bool) // files modified (Write/Edit)
		readFiles    = make(map[string]bool) // files only read
		userMessages []string                // collect user text messages for summary fallback
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}

		// Skip non-message entries.
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}

		// Skip sidechain entries.
		if entry.IsSidechain {
			continue
		}

		// Extract session ID from first entry.
		if sessionID == "" && entry.SessionID != "" {
			sessionID = entry.SessionID
		}

		// Extract branch from first non-empty gitBranch.
		if branch == "" && entry.GitBranch != "" {
			branch = entry.GitBranch
		}

		// Extract timestamp from first entry.
		if createdAt.IsZero() && entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
				createdAt = t
			} else if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				createdAt = t
			}
		}

		if entry.Message == nil {
			continue
		}

		// Collect user text messages for summary extraction.
		if entry.Message.Role == "user" {
			if text := extractUserText(entry.Message.Content); text != "" {
				userMessages = append(userMessages, text)
			}
		}

		// Extract content and files from assistant messages.
		if entry.Message.Role == "assistant" {
			blocks := parseContentBlocks(entry.Message.Content)
			for _, block := range blocks {
				switch block.Type {
				case "text":
					if block.Text != "" {
						if content.Len() > 0 {
							content.WriteByte('\n')
						}
						content.WriteString(block.Text)
					}
				case "tool_use":
					if len(block.Input) > 0 {
						var ti toolInput
						if err := json.Unmarshal(block.Input, &ti); err == nil && ti.FilePath != "" {
							relPath := toRelativePath(ti.FilePath, repoRoot)
							if writeToolNames[block.Name] {
								writeFiles[relPath] = true
							} else if block.Name == "Read" {
								readFiles[relPath] = true
							}
						}
					}
				}
				// Skip tool_result, thinking blocks
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Session{}, fmt.Errorf("scan transcript: %w", err)
	}

	if sessionID == "" {
		return Session{}, fmt.Errorf("no session ID found in transcript")
	}

	summary = extractBestSummary(userMessages, content.String())

	// Build files list: written files first, then read-only files, capped.
	files := buildFilesList(writeFiles, readFiles)

	return Session{
		ID:        sessionID,
		Summary:   summary,
		Content:   content.String(),
		Files:     files,
		Branch:    branch,
		Status:    "succeeded",
		CreatedAt: createdAt,
	}, nil
}

// extractBestSummary picks the best summary from user messages.
// Skips non-substantive messages (tool continuations, interruptions, short acks).
// Falls back to first substantive line from assistant content if no good user message.
func extractBestSummary(userMessages []string, assistantContent string) string {
	for _, msg := range userMessages {
		if isSubstantiveMessage(msg) {
			return truncateSummary(msg)
		}
	}

	// No substantive user message — extract from assistant content.
	if assistantContent != "" {
		if line := firstSubstantiveLine(assistantContent); line != "" {
			return truncateSummary(line)
		}
	}

	if len(userMessages) > 0 {
		return truncateSummary(userMessages[0])
	}

	return "(no summary)"
}

// isSubstantiveMessage returns true if a user message is a real task description,
// not a tool continuation or short acknowledgement.
func isSubstantiveMessage(text string) bool {
	// Skip common non-substantive patterns.
	lower := strings.ToLower(strings.TrimSpace(text))

	// Tool use continuations from Claude Code.
	if strings.HasPrefix(lower, "[request interrupted") {
		return false
	}
	if strings.HasPrefix(lower, "[tool use") {
		return false
	}

	// Very short messages are usually acks ("ok", "yes", "continue", "y").
	if len(lower) < 10 {
		return false
	}

	return true
}

// firstSubstantiveLine returns the first non-preamble line from assistant content.
func firstSubstantiveLine(content string) string {
	lines := strings.SplitN(content, "\n", 20) // check first 20 lines
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isPreamble(line) {
			continue
		}
		return line
	}
	return ""
}

// isPreamble detects filler opening lines that don't convey task substance.
var preamblePrefixes = []string{
	"i'll ",
	"i will ",
	"let me ",
	"let's ",
	"sure,",
	"sure!",
	"okay,",
	"ok,",
	"alright,",
	"now let me ",
	"now let's ",
	"now i'll ",
	"now i have ",
	"now i need ",
	"first,",
	"first let me ",
	"i need to ",
	"i'm going to ",
	"good, i have ",
	"good. i have ",
	"good, now ",
	"good. now ",
	"great, ",
	"great. ",
	"excellent, ",
	"excellent. ",
	"perfect, ",
	"perfect. ",
}

func isPreamble(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range preamblePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// buildFilesList combines written and read files, prioritizing writes, capped at maxFiles.
func buildFilesList(writeFiles, readFiles map[string]bool) []string {
	// Written files first (sorted for deterministic output).
	written := make([]string, 0, len(writeFiles))
	for f := range writeFiles {
		written = append(written, f)
	}
	sort.Strings(written)

	// Read-only files (not in writeFiles).
	readOnly := make([]string, 0, len(readFiles))
	for f := range readFiles {
		if !writeFiles[f] {
			readOnly = append(readOnly, f)
		}
	}
	sort.Strings(readOnly)

	files := append(written, readOnly...)
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	return files
}

// extractUserText gets the text content from a user message, returning empty
// string if the message contains only tool results / no text.
func extractUserText(raw json.RawMessage) string {
	// Try as plain string first.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}

	// Try as array of content blocks — collect text blocks only.
	blocks := parseContentBlocks(raw)
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			return strings.TrimSpace(b.Text)
		}
	}

	return ""
}

// syntheticPrefixes are messages generated by Claude Code (e.g. when the user
// approves a plan), not typed by the user. The real task description follows
// on subsequent lines.
var syntheticPrefixes = []string{
	"implement the following plan:",
	"implement this plan:",
}

func truncateSummary(text string) string {
	lines := strings.SplitN(text, "\n", 10)
	first := strings.TrimSpace(lines[0])

	// If the first line is a known synthetic prefix from Claude Code, skip to
	// the next non-empty line which contains the actual task description.
	if len(lines) > 1 && isSyntheticPrefix(first) {
		for _, line := range lines[1:] {
			candidate := strings.TrimSpace(line)
			candidate = strings.TrimLeft(candidate, "# ") // strip markdown headings
			if candidate != "" {
				first = candidate
				break
			}
		}
	}

	if len(first) > 200 {
		first = first[:200]
	}
	return first
}

func isSyntheticPrefix(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	for _, prefix := range syntheticPrefixes {
		if lower == prefix {
			return true
		}
	}
	return false
}

// parseContentBlocks parses content that may be a string or an array of blocks.
func parseContentBlocks(raw json.RawMessage) []contentBlock {
	// Try as array of blocks.
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}

	// Try as plain string.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil && text != "" {
		return []contentBlock{{Type: "text", Text: text}}
	}

	return nil
}

// toRelativePath strips the repo root from an absolute path.
func toRelativePath(absPath, repoRoot string) string {
	if repoRoot == "" {
		return absPath
	}
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return absPath
	}
	return rel
}
