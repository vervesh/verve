package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joshjon/kit/log"
	"github.com/urfave/cli/v2"

	"github.com/vervesh/verve/internal/app"
	"github.com/vervesh/verve/internal/keymanager"
	"github.com/vervesh/verve/internal/setting"
	"github.com/vervesh/verve/internal/worker"
)

// Version information (injected by goreleaser)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := log.NewLogger(log.WithDevelopment())

	sharedFlags := []cli.Flag{
		&cli.BoolFlag{
			Name:    "github-insecure-skip-verify",
			EnvVars: []string{"GITHUB_INSECURE_SKIP_VERIFY"},
		},
	}

	apiFlags := []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   7400,
			EnvVars: []string{"PORT"},
		},
		&cli.StringFlag{
			Name:    "encryption-key",
			EnvVars: []string{"ENCRYPTION_KEY"},
		},
		&cli.StringFlag{
			Name:    "ui",
			EnvVars: []string{"UI"},
			Usage:   "Enable embedded UI (true/false/auto). Auto enables UI in combined mode.",
			Value:   "auto",
		},
		&cli.StringFlag{
			Name:    "sqlite-dir",
			EnvVars: []string{"SQLITE_DIR"},
		},
		&cli.StringFlag{
			Name:    "turso-dsn",
			EnvVars: []string{"TURSO_DSN"},
			Usage:   "Turso/libSQL database URL (e.g. libsql://db-name.turso.io?authToken=...)",
		},
		&cli.StringFlag{
			Name:    "cors-origins",
			EnvVars: []string{"CORS_ORIGINS"},
			Value:   "http://localhost:5173,http://localhost:8080",
		},
		&cli.DurationFlag{
			Name:    "task-timeout",
			EnvVars: []string{"TASK_TIMEOUT"},
			Value:   5 * time.Minute,
		},
		&cli.DurationFlag{
			Name:    "log-retention",
			EnvVars: []string{"LOG_RETENTION"},
		},
		&cli.StringFlag{
			Name:    "claude-models",
			EnvVars: []string{"CLAUDE_MODELS"},
		},
	}

	workerFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "api-url",
			EnvVars: []string{"API_URL"},
			Value:   "http://localhost:7400",
		},
		&cli.StringFlag{
			Name:    "anthropic-api-key",
			EnvVars: []string{"ANTHROPIC_API_KEY"},
		},
		&cli.StringFlag{
			Name:    "anthropic-base-url",
			EnvVars: []string{"ANTHROPIC_BASE_URL"},
		},
		&cli.StringFlag{
			Name:    "claude-code-oauth-token",
			EnvVars: []string{"CLAUDE_CODE_OAUTH_TOKEN"},
		},
		&cli.StringFlag{
			Name:    "agent-image",
			EnvVars: []string{"AGENT_IMAGE"},
			Value:   "verve:base",
		},
		&cli.IntFlag{
			Name:    "max-concurrent-tasks",
			EnvVars: []string{"MAX_CONCURRENT_TASKS"},
			Value:   3,
		},
		&cli.BoolFlag{
			Name:    "dry-run",
			EnvVars: []string{"DRY_RUN"},
		},
		&cli.BoolFlag{
			Name:    "strip-anthropic-beta-headers",
			EnvVars: []string{"STRIP_ANTHROPIC_BETA_HEADERS"},
		},
		&cli.BoolFlag{
			Name:    "cache",
			EnvVars: []string{"CACHE"},
			Usage:   "Mount a host volume for dependency caching between agent runs",
			Value:   true,
		},
		&cli.BoolFlag{
			Name:    "tome",
			EnvVars: []string{"TOME"},
			Usage:   "Enable tome session memory in agent containers",
		},
		&cli.StringFlag{
			Name:    "cache-dir",
			EnvVars: []string{"CACHE_DIR"},
			Usage:   "Host directory for dependency cache volume",
			Value:   worker.DefaultCacheDir(),
		},
	}

	cliApp := &cli.App{
		Name:    "verve",
		Usage:   "AI agent orchestrator — runs API server and worker",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Flags: concat(sharedFlags, apiFlags, workerFlags),
		Action: func(c *cli.Context) error {
			return runCombined(ctx, c, logger)
		},
		Commands: []*cli.Command{
			{
				Name:  "api",
				Usage: "Run the API server only",
				Flags: concat(sharedFlags, apiFlags),
				Action: func(c *cli.Context) error {
					return runAPI(ctx, c, logger)
				},
			},
			{
				Name:  "worker",
				Usage: "Run the worker only",
				Flags: concat(sharedFlags, workerFlags),
				Action: func(c *cli.Context) error {
					return runWorker(ctx, c, logger)
				},
			},
		},
	}

	if err := cliApp.RunContext(ctx, os.Args); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

// runCombined starts both the API server and worker in the same process.
func runCombined(ctx context.Context, c *cli.Context, logger log.Logger) error {
	// Validate worker auth.
	if !c.Bool("dry-run") && c.String("anthropic-api-key") == "" && c.String("claude-code-oauth-token") == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN is required (or set DRY_RUN=true)")
	}

	// Resolve encryption key (auto-generate for local dev).
	encryptionKey, err := keymanager.ResolveEncryptionKey(c.String("encryption-key"), logger)
	if err != nil {
		return err
	}

	apiCfg := buildAPIConfig(c, encryptionKey, true)
	workerCfg := buildWorkerConfig(c)

	// In combined mode, worker always talks to the co-located API.
	port := c.Int("port")
	workerCfg.APIURL = fmt.Sprintf("http://localhost:%d", port)

	logWorkerConfig(logger, workerCfg)

	// Start API server in background.
	apiErrs := make(chan error, 1)
	go func() {
		apiErrs <- app.Run(ctx, logger, apiCfg)
	}()

	// Wait for API to become healthy.
	healthURL := fmt.Sprintf("http://localhost:%d/healthz", port)
	if err := waitHealthy(ctx, healthURL, 15, time.Second); err != nil {
		return fmt.Errorf("API server failed to become healthy: %w", err)
	}

	// Start worker.
	w, err := worker.New(workerCfg, logger)
	if err != nil {
		return fmt.Errorf("create worker: %w", err)
	}
	defer func() { _ = w.Close() }()

	workerErrs := make(chan error, 1)
	go func() {
		workerErrs <- w.Run(ctx)
	}()

	// Wait for either to exit.
	select {
	case err := <-apiErrs:
		if err != nil {
			return fmt.Errorf("api server: %w", err)
		}
		return nil
	case err := <-workerErrs:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("worker: %w", err)
		}
		return nil
	case <-ctx.Done():
		return nil
	}
}

// runAPI starts only the API server.
func runAPI(ctx context.Context, c *cli.Context, logger log.Logger) error {
	encryptionKey := c.String("encryption-key")
	if encryptionKey == "" {
		logger.Warn("encryption key not set, github token storage will be unavailable")
	}

	cfg := buildAPIConfig(c, encryptionKey, false)

	if cfg.GitHubInsecureSkipVerify {
		logger.Warn("tls certificate verification disabled for github api calls", "config.github_insecure_skip_verify", true)
	}

	return app.Run(ctx, logger, cfg)
}

// runWorker starts only the worker.
func runWorker(ctx context.Context, c *cli.Context, logger log.Logger) error {
	cfg := buildWorkerConfig(c)

	if !cfg.DryRun && cfg.AnthropicAPIKey == "" && cfg.ClaudeCodeOAuthToken == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN is required (or set DRY_RUN=true)")
	}

	if cfg.GitHubInsecureSkipVerify {
		logger.Warn("tls certificate verification disabled for github operations in agent containers", "config.github_insecure_skip_verify", true)
	}

	if cfg.StripAnthropicBetaHeaders {
		logger.Info("agent containers will strip anthropic-beta headers via local reverse proxy", "config.strip_anthropic_beta_headers", true)
	}

	logWorkerConfig(logger, cfg)

	w, err := worker.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("create worker: %w", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func buildAPIConfig(c *cli.Context, encryptionKey string, combined bool) app.Config {
	// Determine UI setting.
	uiFlag := c.String("ui")
	var ui bool
	switch uiFlag {
	case "true":
		ui = true
	case "false":
		ui = false
	default: // "auto"
		ui = combined
	}

	// Determine SQLite directory.
	sqliteDir := c.String("sqlite-dir")
	if sqliteDir == "" && combined {
		// Default to persistent storage in combined mode.
		dataDir, err := dataHome()
		if err == nil {
			sqliteDir = dataDir
		}
	}

	cfg := app.Config{
		Port:                     c.Int("port"),
		UI:                       ui,
		EncryptionKey:            encryptionKey,
		GitHubInsecureSkipVerify: c.Bool("github-insecure-skip-verify"),
		SQLiteDir:                sqliteDir,
		TursoDSN:                 c.String("turso-dsn"),
		CorsOrigins:              parseCorsOrigins(c.String("cors-origins")),
		TaskTimeout:              c.Duration("task-timeout"),
		LogRetention:             c.Duration("log-retention"),
	}

	if models := c.String("claude-models"); models != "" {
		cfg.Models = setting.ParseModelsEnv(models)
	}

	return cfg
}

func buildWorkerConfig(c *cli.Context) worker.Config {
	return worker.Config{
		APIURL:                    c.String("api-url"),
		AnthropicAPIKey:           c.String("anthropic-api-key"),
		AnthropicBaseURL:          c.String("anthropic-base-url"),
		ClaudeCodeOAuthToken:      c.String("claude-code-oauth-token"),
		AgentImage:                c.String("agent-image"),
		MaxConcurrentTasks:        c.Int("max-concurrent-tasks"),
		DryRun:                    c.Bool("dry-run"),
		GitHubInsecureSkipVerify:  c.Bool("github-insecure-skip-verify"),
		StripAnthropicBetaHeaders: c.Bool("strip-anthropic-beta-headers"),
		CacheEnabled:              c.Bool("cache"),
		CacheDir:                  c.String("cache-dir"),
		TomeEnabled:               c.Bool("tome"),
	}
}

func logWorkerConfig(logger log.Logger, cfg worker.Config) {
	authMethod := "api_key"
	if cfg.ClaudeCodeOAuthToken != "" {
		authMethod = "oauth"
	}
	logger.Info("worker configured",
		"worker.api_url", cfg.APIURL,
		"worker.auth_method", authMethod,
		"worker.agent_image", cfg.AgentImage,
		"worker.max_concurrent", cfg.MaxConcurrentTasks,
		"worker.dry_run", cfg.DryRun,
		"worker.cache_enabled", cfg.CacheEnabled,
		"worker.cache_dir", cfg.CacheDir,
		"worker.tome_enabled", cfg.TomeEnabled,
	)
}

func parseCorsOrigins(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// dataHome returns the default data directory (~/.local/share/verve).
func dataHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "verve")
	return dir, nil
}

// waitHealthy polls the given URL until it returns 200 or the attempts are exhausted.
func waitHealthy(ctx context.Context, url string, attempts int, interval time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for range attempts {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("health check at %s failed after %d attempts", url, attempts)
}

func concat(slices ...[]cli.Flag) []cli.Flag {
	var out []cli.Flag
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}
