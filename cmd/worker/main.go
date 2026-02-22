package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/joshjon/kit/log"

	"verve/internal/worker"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := log.NewLogger(log.WithDevelopment())

	// Load configuration from environment
	cfg := worker.Config{
		APIURL:                    getEnvOrDefault("API_URL", "http://localhost:7400"),
		AnthropicAPIKey:           os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicBaseURL:          os.Getenv("ANTHROPIC_BASE_URL"),
		ClaudeCodeOAuthToken:      os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
		AgentImage:                getEnvOrDefault("AGENT_IMAGE", "verve-agent:latest"),
		MaxConcurrentTasks:        getEnvOrDefaultInt(logger, "MAX_CONCURRENT_TASKS", 3),
		DryRun:                    envBool("DRY_RUN"),
		GitHubInsecureSkipVerify:  envBool("GITHUB_INSECURE_SKIP_VERIFY"),
		StripAnthropicBetaHeaders: envBool("STRIP_ANTHROPIC_BETA_HEADERS"),
	}

	// Validate required configuration — need at least one auth method
	if !cfg.DryRun && cfg.AnthropicAPIKey == "" && cfg.ClaudeCodeOAuthToken == "" {
		logger.Error("ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN is required (or set DRY_RUN=true)")
		os.Exit(1)
	}

	authMethod := "api_key"
	if cfg.ClaudeCodeOAuthToken != "" {
		authMethod = "oauth"
	}

	if cfg.GitHubInsecureSkipVerify {
		logger.Warn("GITHUB_INSECURE_SKIP_VERIFY is enabled — TLS certificate verification is disabled for GitHub operations in agent containers")
	}

	if cfg.StripAnthropicBetaHeaders {
		logger.Info("STRIP_ANTHROPIC_BETA_HEADERS is enabled — agent containers will strip anthropic-beta headers via local reverse proxy")
	}

	logger.Info("worker configured",
		"api_url", cfg.APIURL,
		"auth", authMethod,
		"image", cfg.AgentImage,
		"max_concurrent", cfg.MaxConcurrentTasks,
		"dry_run", cfg.DryRun,
	)

	w, err := worker.New(cfg, logger)
	if err != nil {
		logger.Error("failed to create worker", "error", err)
		os.Exit(1)
	}
	defer func() { _ = w.Close() }()

	if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker error", "error", err)
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func envBool(key string) bool {
	return os.Getenv(key) == "true"
}

func getEnvOrDefaultInt(logger log.Logger, key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
		logger.Warn("invalid integer for env var, using default", "key", key, "default", defaultValue)
	}
	return defaultValue
}
