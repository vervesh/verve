package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joshjon/kit/log"
	"github.com/vervesh/verve/internal/redact"
)

const (
	workTypeEpic         = "epic"
	workTypeSetup        = "setup"
	workTypeSetupReview  = "setup-review"
	workTypeConversation = "conversation"
)

// DefaultCacheDir returns the default host directory for caching dependencies between agent runs.
// Always uses ~/.cache/verve for consistent, discoverable placement across platforms.
func DefaultCacheDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "verve")
	}
	return filepath.Join(os.TempDir(), "verve-cache")
}

// Config holds the worker configuration
type Config struct {
	APIURL                    string
	AnthropicAPIKey           string // API key auth (pay-per-use)
	AnthropicBaseURL          string // Custom base URL for Anthropic API (e.g. for proxies or self-hosted endpoints)
	ClaudeCodeOAuthToken      string // OAuth token auth (subscription-based, alternative to API key)
	AgentImage                string // Docker image for agent — defaults to verve:base
	MaxConcurrentTasks        int    // Maximum concurrent tasks (default: 1)
	DryRun                    bool   // Skip Claude and make a dummy change instead
	GitHubInsecureSkipVerify  bool   // Disable TLS certificate verification for GitHub operations in agent containers
	StripAnthropicBetaHeaders bool   // Strip anthropic-beta headers via reverse proxy inside agent containers (for Bedrock proxy compatibility)
	CacheEnabled              bool   // Mount a host volume for dependency caching between agent runs (default: true)
	CacheDir                  string // Host directory for cache volume (default: ~/.cache/verve)
}

type Task struct {
	ID                 string   `json:"id"`
	Number             int      `json:"number"`
	RepoID             string   `json:"repo_id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Status             string   `json:"status"`
	Attempt            int      `json:"attempt"`
	MaxAttempts        int      `json:"max_attempts"`
	RetryReason        string   `json:"retry_reason,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	RetryContext       string   `json:"retry_context,omitempty"`
	AgentStatus        string   `json:"agent_status,omitempty"`
	CostUSD            float64  `json:"cost_usd"`
	MaxCostUSD         float64  `json:"max_cost_usd,omitempty"`
	SkipPR             bool     `json:"skip_pr"`
	DraftPR            bool     `json:"draft_pr"`
	Model              string   `json:"model,omitempty"`
}

type Epic struct {
	ID             string          `json:"id"`
	RepoID         string          `json:"repo_id"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	PlanningPrompt string          `json:"planning_prompt,omitempty"`
	Model          string          `json:"model,omitempty"`
	Feedback       *string         `json:"feedback,omitempty"`
	ProposedTasks  json.RawMessage `json:"proposed_tasks,omitempty"`
}

type Setup struct {
	TaskID   string `json:"task_id"`
	RepoID   string `json:"repo_id"`
	FullName string `json:"full_name"`
}

type Conversation struct {
	ID             string          `json:"id"`
	RepoID         string          `json:"repo_id"`
	Title          string          `json:"title"`
	Messages       json.RawMessage `json:"messages"`
	PendingMessage string          `json:"pending_message"`
	Model          string          `json:"model,omitempty"`
}

// PollResponse is a discriminated union returned by the unified poll endpoint.
type PollResponse struct {
	Type         string        `json:"type"` // "task", "epic", "setup", "conversation", or "stop"
	Task         *Task         `json:"task,omitempty"`
	Epic         *Epic         `json:"epic,omitempty"`
	Setup        *Setup        `json:"setup,omitempty"`
	Conversation *Conversation `json:"conversation,omitempty"`
	Stops        []StopSignal  `json:"stops,omitempty"`
	GitHubToken  string        `json:"github_token"`
	RepoFullName string        `json:"repo_full_name"`

	// Repo setup data (injected into agent prompts)
	RepoSummary      string `json:"repo_summary,omitempty"`
	RepoExpectations string `json:"repo_expectations,omitempty"`
	RepoTechStack    string `json:"repo_tech_stack,omitempty"`
}

// StopSignal identifies an entity that should be stopped (mirrors agentapi.StopSignal).
type StopSignal struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

type Worker struct {
	config       Config
	docker       *DockerRunner
	client       *http.Client
	logger       log.Logger
	pollInterval time.Duration

	// Unique identifier for this worker instance
	workerID string

	// Concurrency control
	maxConcurrent int
	semaphore     chan struct{}
	wg            sync.WaitGroup
	activeTasks   int
	activeMu      sync.Mutex

	// Running execution contexts for stop-signal cancellation
	runningCtxsMu sync.Mutex
	runningCtxs   map[string]context.CancelFunc // entityID → cancel
}

func New(cfg Config, logger log.Logger) (*Worker, error) {
	docker, err := NewDockerRunner(cfg.AgentImage, cfg.CacheEnabled, cfg.CacheDir, logger)
	if err != nil {
		return nil, err
	}

	// Default to 1 concurrent task if not specified
	maxConcurrent := cfg.MaxConcurrentTasks
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	return &Worker{
		config:        cfg,
		docker:        docker,
		client:        &http.Client{Timeout: 60 * time.Second},
		logger:        logger,
		pollInterval:  5 * time.Second,
		workerID:      uuid.New().String(),
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		runningCtxs:   make(map[string]context.CancelFunc),
	}, nil
}

func (w *Worker) Close() error {
	return w.docker.Close()
}

func (w *Worker) trackRunning(id string, cancel context.CancelFunc) {
	w.runningCtxsMu.Lock()
	defer w.runningCtxsMu.Unlock()
	w.runningCtxs[id] = cancel
}

func (w *Worker) untrackRunning(id string) {
	w.runningCtxsMu.Lock()
	defer w.runningCtxsMu.Unlock()
	delete(w.runningCtxs, id)
}

func (w *Worker) cancelRunning(id, entityType string) {
	w.runningCtxsMu.Lock()
	cancel, ok := w.runningCtxs[id]
	w.runningCtxsMu.Unlock()
	if ok {
		w.logger.Info("cancelling execution via stop signal", entityType+".id", id)
		cancel()
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("worker starting", "worker.max_concurrent", w.maxConcurrent)

	// Warn if API URL is not HTTPS (tokens will be sent in plaintext)
	if !strings.HasPrefix(w.config.APIURL, "https://") {
		w.logger.Warn("api url is not https, github tokens will be sent in plaintext", "worker.api_url", w.config.APIURL)
	}

	// Ensure agent image exists
	if err := w.docker.EnsureImage(ctx); err != nil {
		return err
	}
	w.logger.Info("agent image verified", "agent.image", w.docker.AgentImage())

	// Start stop-poll goroutine to receive stop signals via dedicated poll channel.
	go w.stopPollLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker shutting down, waiting for active tasks")
			w.wg.Wait()
			w.logger.Info("all tasks completed, worker stopped")
			return ctx.Err()
		default:
		}

		// Try to acquire a semaphore slot (non-blocking check first)
		select {
		case w.semaphore <- struct{}{}:
			// Got a slot, proceed to poll
		default:
			// All slots full, wait a bit before checking again
			time.Sleep(100 * time.Millisecond)
			continue
		}

		poll, err := w.poll(ctx)
		if err != nil {
			// Release slot on error
			<-w.semaphore
			w.logger.Error("error polling for work", "error", err)
			time.Sleep(w.pollInterval)
			continue
		}

		if poll == nil {
			// No work available, release slot and continue polling
			<-w.semaphore
			continue
		}

		// Track active count for logging
		w.activeMu.Lock()
		w.activeTasks++
		activeCount := w.activeTasks
		w.activeMu.Unlock()

		// Dispatch based on work type
		executeFunc := func(p *PollResponse) {
			switch p.Type {
			case workTypeEpic:
				w.logger.Info("claimed epic",
					"epic.id", p.Epic.ID,
					"repo.full_name", p.RepoFullName,
					"worker.active_tasks", activeCount,
					"epic.title", p.Epic.Title,
				)
				w.executeEpicPlanning(ctx, p)
			case workTypeConversation:
				w.logger.Info("claimed conversation",
					"conversation.id", p.Conversation.ID,
					"repo.full_name", p.RepoFullName,
					"worker.active_tasks", activeCount,
					"conversation.title", p.Conversation.Title,
				)
				w.executeConversation(ctx, p)
			case workTypeSetup, workTypeSetupReview:
				w.logger.Info("claimed setup work",
					"setup.task_id", p.Setup.TaskID,
					"setup.repo_id", p.Setup.RepoID,
					"setup.type", p.Type,
					"repo.full_name", p.RepoFullName,
					"worker.active_tasks", activeCount,
				)
				w.executeSetup(ctx, p)
			default:
				w.logger.Info("claimed task",
					"task.id", p.Task.ID,
					"repo.full_name", p.RepoFullName,
					"worker.active_tasks", activeCount,
					"worker.max_concurrent", w.maxConcurrent,
					"task.description", p.Task.Description,
				)
				w.executeTask(ctx, p)
			}
		}

		if w.maxConcurrent > 1 {
			w.wg.Add(1)
			go func(p *PollResponse) {
				defer w.wg.Done()
				defer func() {
					<-w.semaphore
					w.activeMu.Lock()
					w.activeTasks--
					w.activeMu.Unlock()
				}()
				executeFunc(p)
			}(poll)
		} else {
			executeFunc(poll)
			<-w.semaphore
			w.activeMu.Lock()
			w.activeTasks--
			w.activeMu.Unlock()
		}
	}
}

func (w *Worker) poll(ctx context.Context) (*PollResponse, error) {
	pollURL := w.config.APIURL + "/api/v1/agent/poll"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	// Send worker metadata as query parameters for server-side tracking
	w.activeMu.Lock()
	activeTasks := w.activeTasks
	w.activeMu.Unlock()

	q := req.URL.Query()
	q.Set("worker_id", w.workerID)
	q.Set("max_concurrent", fmt.Sprintf("%d", w.maxConcurrent))
	q.Set("active_tasks", fmt.Sprintf("%d", activeTasks))
	req.URL.RawQuery = q.Encode()

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var envelope struct {
		Data PollResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}

	return &envelope.Data, nil
}

func (w *Worker) stopPollLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stops, err := w.pollForStops(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("error polling for stops", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, s := range stops {
			w.cancelRunning(s.EntityID, s.EntityType)
		}
	}
}

func (w *Worker) pollForStops(ctx context.Context) ([]StopSignal, error) {
	pollURL := w.config.APIURL + "/api/v1/agent/poll?accept=stop"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var envelope struct {
		Data struct {
			Stops []StopSignal `json:"stops"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}

	return envelope.Data.Stops, nil
}

// logStreamer buffers log lines and periodically sends them to the API server
type logStreamer struct {
	worker         *Worker
	taskID         string
	epicID         string
	conversationID string
	attempt        int
	ctx            context.Context
	buffer         []string
	mu             sync.Mutex
	done           chan struct{}
	flushed        chan struct{}
	interval       time.Duration
	batchSize      int
}

func newLogStreamer(ctx context.Context, w *Worker, taskID string, attempt int) *logStreamer {
	ls := &logStreamer{
		worker:    w,
		taskID:    taskID,
		attempt:   attempt,
		ctx:       ctx,
		buffer:    make([]string, 0, 100),
		done:      make(chan struct{}),
		flushed:   make(chan struct{}),
		interval:  2 * time.Second,
		batchSize: 50,
	}
	go ls.flushLoop()
	return ls
}

func newEpicLogStreamer(ctx context.Context, w *Worker, epicID string) *logStreamer {
	ls := &logStreamer{
		worker:    w,
		epicID:    epicID,
		ctx:       ctx,
		buffer:    make([]string, 0, 100),
		done:      make(chan struct{}),
		flushed:   make(chan struct{}),
		interval:  2 * time.Second,
		batchSize: 50,
	}
	go ls.flushLoop()
	return ls
}

func newConversationLogStreamer(ctx context.Context, w *Worker, conversationID string) *logStreamer {
	ls := &logStreamer{
		worker:         w,
		conversationID: conversationID,
		ctx:            ctx,
		buffer:         make([]string, 0, 100),
		done:           make(chan struct{}),
		flushed:        make(chan struct{}),
		interval:       2 * time.Second,
		batchSize:      50,
	}
	go ls.flushLoop()
	return ls
}

// AddLine adds a log line to the buffer (thread-safe).
// Sensitive data (API keys, tokens, passwords, etc.) is automatically redacted.
func (ls *logStreamer) AddLine(line string) {
	line = redact.Line(line)
	ls.mu.Lock()
	ls.buffer = append(ls.buffer, line)
	shouldFlush := len(ls.buffer) >= ls.batchSize
	ls.mu.Unlock()

	// Flush immediately if buffer is large
	if shouldFlush {
		ls.flush()
	}
}

// Stop signals the streamer to stop and waits for final flush
func (ls *logStreamer) Stop() {
	close(ls.done)
	<-ls.flushed
}

func (ls *logStreamer) flushLoop() {
	defer close(ls.flushed)

	ticker := time.NewTicker(ls.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ls.flush()
		case <-ls.done:
			// Final flush
			ls.flush()
			return
		}
	}
}

func (ls *logStreamer) flush() {
	ls.mu.Lock()
	if len(ls.buffer) == 0 {
		ls.mu.Unlock()
		return
	}
	// Take ownership of the buffer
	toSend := ls.buffer
	ls.buffer = make([]string, 0, 100)
	ls.mu.Unlock()

	// Send to API server
	switch {
	case ls.taskID != "":
		if err := ls.worker.sendLogs(ls.ctx, ls.taskID, ls.attempt, toSend); err != nil {
			ls.worker.logger.Error("failed to send logs", "task.id", ls.taskID, "error", err)
		}
	case ls.epicID != "":
		if err := ls.worker.sendEpicLogs(ls.ctx, ls.epicID, toSend); err != nil {
			ls.worker.logger.Error("failed to send epic logs", "epic.id", ls.epicID, "error", err)
		}
	case ls.conversationID != "":
		if err := ls.worker.sendConversationLogs(ls.ctx, ls.conversationID, toSend); err != nil {
			ls.worker.logger.Error("failed to send conversation logs", "conversation.id", ls.conversationID, "error", err)
		}
	}
}

func (w *Worker) executeTask(ctx context.Context, poll *PollResponse) {
	task := poll.Task
	githubToken := poll.GitHubToken
	repoFullName := poll.RepoFullName
	taskLogger := w.logger.With("task.id", task.ID)

	// Create log streamer for real-time log streaming
	streamer := newLogStreamer(ctx, w, task.ID, task.Attempt)

	// Track PR info, branch info, and agent markers
	var prURL string
	var prNumber int
	var branchName string
	var agentStatus string
	var costUSD float64
	var noChanges bool
	var rateLimited bool
	var transientError bool
	var authError bool
	var markerMu sync.Mutex

	// Log callback - called from Docker log streaming goroutine
	onLog := func(line string) {
		taskLogger.Debug("agent output", "agent.line", line)
		streamer.AddLine(line)

		// Strip markdown formatting (e.g. **bold**) that the agent
		// may wrap around marker lines.
		cleanLine := strings.TrimRight(strings.TrimLeft(line, "*"), "*")

		// Parse PR marker
		if strings.HasPrefix(cleanLine, "VERVE_PR_CREATED:") {
			jsonStr := strings.TrimPrefix(cleanLine, "VERVE_PR_CREATED:")
			var prInfo struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &prInfo); err == nil {
				markerMu.Lock()
				prURL = prInfo.URL
				prNumber = prInfo.Number
				markerMu.Unlock()
				taskLogger.Info("captured pr", "pr.url", prURL, "pr.number", prNumber)
			}
		}

		// Parse PR updated marker (retry with existing PR)
		if strings.HasPrefix(cleanLine, "VERVE_PR_UPDATED:") {
			jsonStr := strings.TrimPrefix(cleanLine, "VERVE_PR_UPDATED:")
			var prInfo struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &prInfo); err == nil {
				markerMu.Lock()
				prURL = prInfo.URL
				prNumber = prInfo.Number
				markerMu.Unlock()
				taskLogger.Info("captured pr update", "pr.url", prURL, "pr.number", prNumber)
			}
		}

		// Parse branch pushed marker (skip-PR mode)
		if strings.HasPrefix(cleanLine, "VERVE_BRANCH_PUSHED:") {
			jsonStr := strings.TrimPrefix(cleanLine, "VERVE_BRANCH_PUSHED:")
			var branchInfo struct {
				Branch string `json:"branch"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &branchInfo); err == nil {
				markerMu.Lock()
				branchName = branchInfo.Branch
				markerMu.Unlock()
				taskLogger.Info("captured branch", "task.branch", branchName)
			}
		}

		// Parse agent status marker
		if strings.HasPrefix(cleanLine, "VERVE_STATUS:") {
			statusJSON := strings.TrimPrefix(cleanLine, "VERVE_STATUS:")
			markerMu.Lock()
			agentStatus = statusJSON
			markerMu.Unlock()
			taskLogger.Info("captured agent status")
		}

		// Parse no-changes marker
		if strings.HasPrefix(cleanLine, "VERVE_NO_CHANGES:") {
			markerMu.Lock()
			noChanges = true
			markerMu.Unlock()
			taskLogger.Info("agent reported no changes needed")
		}

		// Parse cost marker
		if strings.HasPrefix(cleanLine, "VERVE_COST:") {
			costStr := strings.TrimPrefix(cleanLine, "VERVE_COST:")
			var cost float64
			if _, err := fmt.Sscanf(costStr, "%f", &cost); err == nil {
				markerMu.Lock()
				costUSD = cost
				markerMu.Unlock()
				taskLogger.Info("captured cost", "task.cost_usd", cost)
			}
		}

		// Detect Claude rate limit or session max usage errors
		if isRateLimitError(line) {
			markerMu.Lock()
			rateLimited = true
			markerMu.Unlock()
			taskLogger.Warn("detected claude rate limit or max usage error")
		}

		// Detect transient infrastructure errors (network, DNS, timeouts)
		if isTransientError(line) {
			markerMu.Lock()
			transientError = true
			markerMu.Unlock()
			taskLogger.Warn("detected transient infrastructure error", "agent.line", line)
		}

		// Detect authentication errors (expired/invalid API key)
		if isAuthError(line) {
			markerMu.Lock()
			authError = true
			markerMu.Unlock()
			taskLogger.Warn("detected authentication error", "agent.line", line)
		}
	}

	// Create agent config from worker config + server-provided credentials
	agentCfg := AgentConfig{
		WorkType:                  "task",
		TaskID:                    task.ID,
		TaskNumber:                task.Number,
		TaskTitle:                 task.Title,
		TaskDescription:           task.Description,
		GitHubToken:               githubToken,
		GitHubRepo:                repoFullName,
		AnthropicAPIKey:           w.config.AnthropicAPIKey,
		AnthropicBaseURL:          w.config.AnthropicBaseURL,
		ClaudeCodeOAuthToken:      w.config.ClaudeCodeOAuthToken,
		ClaudeModel:               task.Model,
		DryRun:                    w.config.DryRun,
		GitHubInsecureSkipVerify:  w.config.GitHubInsecureSkipVerify,
		StripAnthropicBetaHeaders: w.config.StripAnthropicBetaHeaders,
		SkipPR:                    task.SkipPR,
		DraftPR:                   task.DraftPR,
		Attempt:                   task.Attempt,
		RetryReason:               task.RetryReason,
		AcceptanceCriteria:        task.AcceptanceCriteria,
		RetryContext:              task.RetryContext,
		PreviousStatus:            task.AgentStatus,
		RepoSummary:               poll.RepoSummary,
		RepoExpectations:          poll.RepoExpectations,
		RepoTechStack:             poll.RepoTechStack,
	}

	// Create a cancellable context for the agent execution.
	// The stop-poll goroutine or heartbeat safety net can cancel this.
	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()
	w.trackRunning(task.ID, cancelExec)
	defer w.untrackRunning(task.ID)

	// Start heartbeat goroutine
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go w.taskHeartbeatLoop(heartbeatCtx, task.ID, cancelExec)

	// Run the agent with streaming logs
	result := w.docker.RunAgent(execCtx, agentCfg, onLog)

	// Stop heartbeat before completing the task
	cancelHeartbeat()

	// If the execution was cancelled because the task was stopped,
	// don't report completion — the server already moved it to pending.
	if execCtx.Err() != nil && ctx.Err() == nil {
		taskLogger.Info("task execution cancelled (stopped by user)")
		streamer.Stop()
		return
	}

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	// Get captured marker values
	markerMu.Lock()
	capturedPRURL := prURL
	capturedPRNumber := prNumber
	capturedBranchName := branchName
	capturedAgentStatus := agentStatus
	capturedCostUSD := costUSD
	capturedNoChanges := noChanges
	capturedRateLimited := rateLimited
	capturedTransientError := transientError
	capturedAuthError := authError
	markerMu.Unlock()

	// Report completion with PR info, agent status, and cost
	switch {
	case result.Error != nil:
		retryable := capturedRateLimited || capturedTransientError || isDockerInfraError(result.Error)
		taskLogger.Error("task failed", "error", result.Error, "task.retryable", retryable)
		_ = w.completeTask(ctx, task.ID, false, result.Error.Error(), "", 0, "", capturedAgentStatus, capturedCostUSD, false, retryable)
	case result.Success:
		// Defense-in-depth: if the agent exited successfully but we detected
		// authentication or rate-limit errors in the logs and no actual work
		// was done (no PR, no branch, no changes), treat it as a failure.
		// This catches the scenario where Claude fails mid-session (e.g. expired
		// API key) but the agent container still exits 0 with "no changes".
		switch {
		case capturedNoChanges && (capturedAuthError || capturedRateLimited):
			errMsg := "agent completed with no changes due to API errors"
			if capturedAuthError {
				errMsg = "agent completed with no changes due to authentication error (check API key)"
			}
			taskLogger.Error("task failed, no changes due to api error", "task.auth_error", capturedAuthError, "task.rate_limited", capturedRateLimited)
			_ = w.completeTask(ctx, task.ID, false, errMsg, "", 0, "", capturedAgentStatus, capturedCostUSD, false, capturedRateLimited)
		case capturedNoChanges:
			taskLogger.Info("task completed, no changes needed")
			_ = w.completeTask(ctx, task.ID, true, "", capturedPRURL, capturedPRNumber, capturedBranchName, capturedAgentStatus, capturedCostUSD, capturedNoChanges, false)
		default:
			taskLogger.Info("task completed successfully")
			_ = w.completeTask(ctx, task.ID, true, "", capturedPRURL, capturedPRNumber, capturedBranchName, capturedAgentStatus, capturedCostUSD, capturedNoChanges, false)
		}
	default:
		errMsg := fmt.Sprintf("exit code %d", result.ExitCode)
		retryable := capturedRateLimited || capturedTransientError
		taskLogger.Error("task failed", "container.exit_code", result.ExitCode, "task.retryable", retryable)
		_ = w.completeTask(ctx, task.ID, false, errMsg, "", 0, "", capturedAgentStatus, capturedCostUSD, false, retryable)
	}
}

func (w *Worker) executeEpicPlanning(ctx context.Context, poll *PollResponse) {
	ep := poll.Epic
	githubToken := poll.GitHubToken
	repoFullName := poll.RepoFullName
	epicLogger := w.logger.With("epic.id", ep.ID)
	epicLogger.Info("starting epic planning", "epic.title", ep.Title)

	// Create log streamer for real-time log streaming
	streamer := newEpicLogStreamer(ctx, w, ep.ID)

	var feedback string
	if ep.Feedback != nil {
		feedback = *ep.Feedback
	}
	var previousPlan string
	if len(ep.ProposedTasks) > 0 {
		previousPlan = string(ep.ProposedTasks)
	}

	agentCfg := AgentConfig{
		WorkType:                  workTypeEpic,
		EpicID:                    ep.ID,
		EpicTitle:                 ep.Title,
		EpicDescription:           ep.Description,
		EpicPlanningPrompt:        ep.PlanningPrompt,
		EpicFeedback:              feedback,
		EpicPreviousPlan:          previousPlan,
		APIURL:                    w.config.APIURL,
		GitHubToken:               githubToken,
		GitHubRepo:                repoFullName,
		AnthropicAPIKey:           w.config.AnthropicAPIKey,
		AnthropicBaseURL:          w.config.AnthropicBaseURL,
		ClaudeCodeOAuthToken:      w.config.ClaudeCodeOAuthToken,
		ClaudeModel:               ep.Model,
		GitHubInsecureSkipVerify:  w.config.GitHubInsecureSkipVerify,
		StripAnthropicBetaHeaders: w.config.StripAnthropicBetaHeaders,
		RepoSummary:               poll.RepoSummary,
		RepoExpectations:          poll.RepoExpectations,
		RepoTechStack:             poll.RepoTechStack,
	}

	// Create a cancellable context for epic execution.
	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()
	w.trackRunning(ep.ID, cancelExec)
	defer w.untrackRunning(ep.ID)

	// Start heartbeat goroutine
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go w.epicHeartbeatLoop(heartbeatCtx, ep.ID, cancelExec)

	// Log callback for epic planning
	onLog := func(line string) {
		epicLogger.Info("epic agent", "agent.line", line)
		streamer.AddLine(line)
	}

	result := w.docker.RunAgent(execCtx, agentCfg, onLog)

	// Stop heartbeat before completing
	cancelHeartbeat()

	// If the execution was cancelled because the epic was stopped,
	// don't report completion — the server already moved it to draft.
	if execCtx.Err() != nil && ctx.Err() == nil {
		epicLogger.Info("epic planning cancelled (stopped by user)")
		streamer.Stop()
		return
	}

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	switch {
	case result.Error != nil:
		epicLogger.Error("epic planning failed", "error", result.Error)
	case result.Success:
		epicLogger.Info("epic planning completed successfully")
	default:
		epicLogger.Error("epic planning container failed", "container.exit_code", result.ExitCode)
	}
}

func (w *Worker) executeSetup(ctx context.Context, poll *PollResponse) {
	setup := poll.Setup
	githubToken := poll.GitHubToken
	repoFullName := poll.RepoFullName
	setupLogger := w.logger.With("setup.task_id", setup.TaskID, "setup.repo_id", setup.RepoID, "setup.type", poll.Type)
	setupLogger.Info("starting repo setup work", "repo.full_name", repoFullName)

	// Create log streamer for real-time log streaming (uses task log endpoint)
	streamer := newLogStreamer(ctx, w, setup.TaskID, 1)

	// Log callback
	onLog := func(line string) {
		setupLogger.Debug("setup agent output", "agent.line", line)
		streamer.AddLine(line)
	}

	// Determine work type: "setup" for initial scan, "setup-review" for AI review of user config
	workType := workTypeSetup
	if poll.Type == workTypeSetupReview {
		workType = workTypeSetupReview
	}

	agentCfg := AgentConfig{
		WorkType:                  workType,
		SetupRepoID:              setup.RepoID,
		TaskID:                    setup.TaskID,
		APIURL:                    w.config.APIURL,
		GitHubToken:               githubToken,
		GitHubRepo:                repoFullName,
		AnthropicAPIKey:           w.config.AnthropicAPIKey,
		AnthropicBaseURL:          w.config.AnthropicBaseURL,
		ClaudeCodeOAuthToken:      w.config.ClaudeCodeOAuthToken,
		ClaudeModel:               "sonnet",
		GitHubInsecureSkipVerify:  w.config.GitHubInsecureSkipVerify,
		StripAnthropicBetaHeaders: w.config.StripAnthropicBetaHeaders,
		RepoSummary:              poll.RepoSummary,
		RepoExpectations:         poll.RepoExpectations,
		RepoTechStack:            poll.RepoTechStack,
	}

	// Start heartbeat goroutine using the setup heartbeat endpoint
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go w.setupHeartbeatLoop(heartbeatCtx, setup.RepoID)

	result := w.docker.RunAgent(ctx, agentCfg, onLog)

	// Stop heartbeat before completing
	cancelHeartbeat()

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	switch {
	case result.Error != nil:
		setupLogger.Error("setup scan failed", "error", result.Error)
		_ = w.completeTask(ctx, setup.TaskID, false, result.Error.Error(), "", 0, "", "", 0, false, false)
	case result.Success:
		setupLogger.Info("setup scan completed successfully")
		// The agent script calls POST /repos/:repo_id/setup-complete directly.
		// Mark the underlying task as closed.
		_ = w.completeTask(ctx, setup.TaskID, true, "", "", 0, "", "", 0, true, false)
	default:
		errMsg := fmt.Sprintf("exit code %d", result.ExitCode)
		setupLogger.Error("setup scan failed", "container.exit_code", result.ExitCode)
		_ = w.completeTask(ctx, setup.TaskID, false, errMsg, "", 0, "", "", 0, false, false)
	}
}

func (w *Worker) executeConversation(ctx context.Context, poll *PollResponse) {
	conv := poll.Conversation
	githubToken := poll.GitHubToken
	repoFullName := poll.RepoFullName
	convLogger := w.logger.With("conversation.id", conv.ID)
	convLogger.Info("starting conversation processing", "conversation.title", conv.Title)

	// Create log streamer for real-time log streaming
	streamer := newConversationLogStreamer(ctx, w, conv.ID)

	// Serialize messages to JSON string for the agent container
	messagesJSON := string(conv.Messages)
	if messagesJSON == "" {
		messagesJSON = "[]"
	}

	agentCfg := AgentConfig{
		WorkType:                  workTypeConversation,
		ConversationID:            conv.ID,
		ConversationTitle:         conv.Title,
		ConversationMessages:      messagesJSON,
		ConversationPendingMessage: conv.PendingMessage,
		APIURL:                    w.config.APIURL,
		GitHubToken:               githubToken,
		GitHubRepo:                repoFullName,
		AnthropicAPIKey:           w.config.AnthropicAPIKey,
		AnthropicBaseURL:          w.config.AnthropicBaseURL,
		ClaudeCodeOAuthToken:      w.config.ClaudeCodeOAuthToken,
		ClaudeModel:               conv.Model,
		GitHubInsecureSkipVerify:  w.config.GitHubInsecureSkipVerify,
		StripAnthropicBetaHeaders: w.config.StripAnthropicBetaHeaders,
		RepoSummary:               poll.RepoSummary,
		RepoExpectations:          poll.RepoExpectations,
		RepoTechStack:             poll.RepoTechStack,
	}

	// Start heartbeat goroutine
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go w.conversationHeartbeatLoop(heartbeatCtx, conv.ID)

	// Log callback for conversation
	onLog := func(line string) {
		convLogger.Debug("conversation agent", "agent.line", line)
		streamer.AddLine(line)
	}

	result := w.docker.RunAgent(ctx, agentCfg, onLog)

	// Stop heartbeat before completing
	cancelHeartbeat()

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	switch {
	case result.Error != nil:
		convLogger.Error("conversation processing failed", "error", result.Error)
		_ = w.completeConversation(ctx, conv.ID, false, "", result.Error.Error())
	case result.Success:
		convLogger.Info("conversation processing completed successfully")
		// The agent script calls POST /agent/conversations/:id/complete directly
		// with the response text, so we don't need to complete here on success.
	default:
		errMsg := fmt.Sprintf("exit code %d", result.ExitCode)
		convLogger.Error("conversation container failed", "container.exit_code", result.ExitCode)
		_ = w.completeConversation(ctx, conv.ID, false, "", errMsg)
	}
}

func (w *Worker) conversationHeartbeatLoop(ctx context.Context, conversationID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	_ = w.sendConversationHeartbeat(ctx, conversationID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.sendConversationHeartbeat(ctx, conversationID)
		}
	}
}

func (w *Worker) sendConversationHeartbeat(ctx context.Context, conversationID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.APIURL+"/api/v1/agent/conversations/"+conversationID+"/heartbeat", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (w *Worker) completeConversation(ctx context.Context, conversationID string, success bool, response, errMsg string) error {
	payload := map[string]interface{}{"success": success}
	if response != "" {
		payload["response"] = response
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.APIURL+"/api/v1/agent/conversations/"+conversationID+"/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) sendConversationLogs(ctx context.Context, conversationID string, logs []string) error {
	body, _ := json.Marshal(map[string]any{"lines": logs})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.APIURL+"/api/v1/agent/conversations/"+conversationID+"/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) setupHeartbeatLoop(ctx context.Context, repoID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	_ = w.sendSetupHeartbeat(ctx, repoID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.sendSetupHeartbeat(ctx, repoID)
		}
	}
}

func (w *Worker) sendSetupHeartbeat(ctx context.Context, repoID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.APIURL+"/api/v1/agent/repos/"+repoID+"/setup-heartbeat", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (w *Worker) sendLogs(ctx context.Context, taskID string, attempt int, logs []string) error {
	body, _ := json.Marshal(map[string]any{"logs": logs, "attempt": attempt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.APIURL+"/api/v1/agent/tasks/"+taskID+"/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) sendEpicLogs(ctx context.Context, epicID string, logs []string) error {
	body, _ := json.Marshal(map[string]any{"lines": logs})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.APIURL+"/api/v1/agent/epics/"+epicID+"/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) taskHeartbeatLoop(ctx context.Context, taskID string, cancelExecution context.CancelFunc) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send initial heartbeat immediately
	if stopped := w.sendTaskHeartbeat(ctx, taskID); stopped {
		w.logger.Info("task was stopped, cancelling execution", "task.id", taskID)
		cancelExecution()
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if stopped := w.sendTaskHeartbeat(ctx, taskID); stopped {
				w.logger.Info("task was stopped, cancelling execution", "task.id", taskID)
				cancelExecution()
				return
			}
		}
	}
}

func (w *Worker) sendTaskHeartbeat(ctx context.Context, taskID string) (stopped bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.APIURL+"/api/v1/agent/tasks/"+taskID+"/heartbeat", http.NoBody)
	if err != nil {
		return false
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	// The server wraps responses in a {"data": ...} envelope.
	// Parse the envelope to check if the task was stopped.
	var result struct {
		Data struct {
			Stopped bool `json:"stopped"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Data.Stopped
}

func (w *Worker) epicHeartbeatLoop(ctx context.Context, epicID string, cancelExecution context.CancelFunc) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	if stopped := w.sendEpicHeartbeat(ctx, epicID); stopped {
		w.logger.Info("epic was stopped, cancelling execution", "epic.id", epicID)
		cancelExecution()
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if stopped := w.sendEpicHeartbeat(ctx, epicID); stopped {
				w.logger.Info("epic was stopped, cancelling execution", "epic.id", epicID)
				cancelExecution()
				return
			}
		}
	}
}

func (w *Worker) sendEpicHeartbeat(ctx context.Context, epicID string) (stopped bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.APIURL+"/api/v1/agent/epics/"+epicID+"/heartbeat", http.NoBody)
	if err != nil {
		return false
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Data struct {
			Stopped bool `json:"stopped"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Data.Stopped
}

func (w *Worker) completeTask(ctx context.Context, taskID string, success bool, errMsg, prURL string, prNumber int, branchName, agentStatus string, costUSD float64, noChanges, retryable bool) error {
	payload := map[string]interface{}{"success": success}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	if prURL != "" {
		payload["pull_request_url"] = prURL
		payload["pr_number"] = prNumber
	}
	if branchName != "" {
		payload["branch_name"] = branchName
	}
	if agentStatus != "" {
		payload["agent_status"] = agentStatus
	}
	if costUSD > 0 {
		payload["cost_usd"] = costUSD
	}
	if noChanges {
		payload["no_changes"] = true
	}
	if retryable {
		payload["retryable"] = true
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.APIURL+"/api/v1/agent/tasks/"+taskID+"/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// rateLimitPatterns are substrings in agent output that indicate Claude rate
// limit or session max usage errors. These are transient and the task should
// be retried after a delay rather than permanently failed.
var rateLimitPatterns = []string{
	"max usage",
	"rate limit",
	"rate_limit",
	"too many requests",
	"overloaded_error",
}

// isRateLimitError checks if a log line indicates a Claude rate limit or
// session max usage error.
func isRateLimitError(line string) bool {
	lower := strings.ToLower(line)
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// transientErrorPatterns are substrings in agent output that indicate
// transient infrastructure errors (network issues, DNS failures, timeouts).
// These are temporary and the task should be retried automatically.
var transientErrorPatterns = []string{
	"could not resolve host",
	"unable to access",
	"unable to look up",
	"connection refused",
	"connection timed out",
	"connection reset by peer",
	"no such host",
	"network is unreachable",
	"temporary failure in name resolution",
	"tls handshake timeout",
	"i/o timeout",
	"unexpected disconnect",
	"the remote end hung up unexpectedly",
	"early eof",
	"ssl_error",
	"gnutls_handshake",
	"failed to connect",
	"couldn't connect to server",
	"couldn't resolve host",
}

// isTransientError checks if a log line indicates a transient infrastructure
// error such as a network failure, DNS issue, or connection timeout.
func isTransientError(line string) bool {
	lower := strings.ToLower(line)
	for _, pattern := range transientErrorPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// authErrorPatterns are substrings in agent output that indicate authentication
// errors (expired or invalid API keys). These are NOT retryable automatically
// since the user needs to fix their credentials.
var authErrorPatterns = []string{
	"authentication_error",
	"invalid x-api-key",
	"invalid api key",
	"invalid_api_key",
	"api key expired",
	"api key is invalid",
	"api key not found",
	"unauthorized",
	"invalid auth",
	"authentication failed",
	"credit balance is too low",
}

// isAuthError checks if a log line indicates an authentication or
// authorization error (expired API key, invalid credentials, etc.).
func isAuthError(line string) bool {
	lower := strings.ToLower(line)
	for _, pattern := range authErrorPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isDockerInfraError checks if a RunResult error is a Docker infrastructure
// error (container creation/start failure) that is likely transient and should
// be retried.
func isDockerInfraError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	infraPatterns := []string{
		"failed to create container",
		"failed to start container",
		"failed to attach logs",
		"error waiting for container",
		"no such container",
		"conflict",
	}
	for _, p := range infraPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
