package app

import (
	"time"

	"github.com/joshjon/verve/internal/setting"
)

// Config holds the API server configuration.
type Config struct {
	Port                       int
	UI                         bool
	SQLiteDir                  string         // Directory for SQLite DB file; if empty, uses in-memory
	TursoDSN                   string         // Turso/libSQL DSN (e.g. "libsql://db-name.turso.io?authToken=...")
	EncryptionKey              string         // Hex-encoded 32-byte key for encrypting secrets at rest
	GitHubInsecureSkipVerify   bool           // Disable TLS certificate verification for GitHub API calls
	CorsOrigins                []string
	TaskTimeout                time.Duration // How long before a running task with no heartbeat is considered stale (default: 5m)
	LogRetention               time.Duration // How long to keep task logs before deleting them (0 = keep forever)
	ConversationRetention      time.Duration // How long before active conversations are auto-archived (default: 7 days, 0 = keep forever)
	Models                     []setting.ModelOption // Available Claude models; if empty, uses DefaultModels
}

// EffectiveModels returns the configured models or the default set.
func (c Config) EffectiveModels() []setting.ModelOption {
	if len(c.Models) > 0 {
		return c.Models
	}
	return setting.DefaultModels
}
