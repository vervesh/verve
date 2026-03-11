// Package redact provides log line redaction to prevent sensitive data leakage.
package redact

import (
	"regexp"
)

const placeholder = "[REDACTED]"

type rule struct {
	pattern     *regexp.Regexp
	replacement string
}

// rules defines redaction patterns. Each pattern uses capture groups where $1, $2 etc.
// in the replacement refer to non-sensitive parts to preserve.
var rules = []rule{
	// Bearer tokens in Authorization headers (e.g. curl -H "Authorization: Bearer sk-...")
	{regexp.MustCompile(`(?i)((?:Authorization|auth)\s*[=:]\s*Bearer\s+)\S+`), "${1}" + placeholder},

	// Generic Authorization header values (Basic, Token, Digest)
	{regexp.MustCompile(`(?i)((?:Authorization|auth)\s*[=:]\s*(?:Basic|Token|Digest)\s+)\S+`), "${1}" + placeholder},

	// API keys in common header patterns (e.g. -H "X-Api-Key: ...")
	{regexp.MustCompile(`(?i)((?:X-Api-Key|X-API-Key|api[_-]?key|apikey)\s*[=:]\s*)\S+`), "${1}" + placeholder},

	// AWS access keys (AKIA...)
	{regexp.MustCompile(`(^|[^A-Za-z0-9])AKIA[0-9A-Z]{16}([^A-Za-z0-9]|$)`), "${1}" + placeholder + "${2}"},

	// AWS secret key values
	{regexp.MustCompile(`(?i)((?:aws_secret_access_key|aws_secret_key|secret_access_key)\s*[=:]\s*)\S+`), "${1}" + placeholder},

	// GitHub tokens (ghp_, gho_, ghu_, ghs_, ghr_)
	{regexp.MustCompile(`(^|[^A-Za-z0-9_])(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}`), "${1}" + placeholder},

	// GitHub fine-grained PATs
	{regexp.MustCompile(`(^|[^A-Za-z0-9_])github_pat_[A-Za-z0-9_]{22,255}`), "${1}" + placeholder},

	// OpenAI API keys (sk-proj..., sk-...)
	{regexp.MustCompile(`(^|[^A-Za-z0-9_-])sk-[A-Za-z0-9]{20,}`), "${1}" + placeholder},

	// Anthropic API keys (sk-ant-...)
	{regexp.MustCompile(`(^|[^A-Za-z0-9_-])sk-ant-[A-Za-z0-9_-]{20,}`), "${1}" + placeholder},

	// Slack tokens (xoxb-, xoxp-, xoxo-, xapp-)
	{regexp.MustCompile(`(^|[^A-Za-z0-9_-])xox[bpoa]-[A-Za-z0-9-]{10,}`), "${1}" + placeholder},
	{regexp.MustCompile(`(^|[^A-Za-z0-9_-])xapp-[A-Za-z0-9-]{10,}`), "${1}" + placeholder},

	// Private keys (PEM format)
	{regexp.MustCompile(`(?i)-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`), placeholder},

	// Connection strings with embedded passwords
	{regexp.MustCompile(`(?i)((?:postgres|mysql|mongodb|redis|amqp)(?:ql)?://[^:]+:)[^@\s]+(@)`), "${1}" + placeholder + "${2}"},

	// Generic secret/token/password/credentials in key=value or key: value patterns.
	// This is intentionally last so more specific patterns match first.
	{regexp.MustCompile(`(?i)((?:secret|token|password|passwd|credentials|private_key|access_key)\s*[=:]\s*)\S+`), "${1}" + placeholder},
}

// Line redacts sensitive data from a single log line.
func Line(line string) string {
	for _, r := range rules {
		line = r.pattern.ReplaceAllString(line, r.replacement)
	}
	return line
}

// Lines redacts sensitive data from a slice of log lines.
func Lines(lines []string) []string {
	redacted := make([]string, len(lines))
	for i, line := range lines {
		redacted[i] = Line(line)
	}
	return redacted
}
