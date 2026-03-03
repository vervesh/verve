package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/joshjon/kit/log"
)

const DefaultAgentImage = "verve:base"

type DockerRunner struct {
	client     *client.Client
	agentImage string
	logger     log.Logger
}

func NewDockerRunner(agentImage string, logger log.Logger) (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	if agentImage == "" {
		agentImage = DefaultAgentImage
	}
	return &DockerRunner{client: cli, agentImage: agentImage, logger: logger}, nil
}

func (d *DockerRunner) Close() error {
	return d.client.Close()
}

// EnsureImage checks if the agent image exists locally
func (d *DockerRunner) EnsureImage(ctx context.Context) error {
	images, err := d.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == d.agentImage {
				return nil
			}
		}
	}

	return fmt.Errorf("agent image %s not found", d.agentImage)
}

// AgentImage returns the configured agent image name
func (d *DockerRunner) AgentImage() string {
	return d.agentImage
}

type RunResult struct {
	Success  bool
	ExitCode int
	Error    error
}

// AgentConfig holds the configuration for running an agent
type AgentConfig struct {
	WorkType string // "task", "epic", or "setup"

	// Task fields
	TaskID               string
	TaskTitle            string
	TaskDescription      string
	SkipPR               bool
	DraftPR              bool
	Attempt              int
	RetryReason          string
	AcceptanceCriteria   []string
	RetryContext         string
	PreviousStatus       string

	// Epic fields
	EpicID             string
	EpicTitle          string
	EpicDescription    string
	EpicPlanningPrompt string
	EpicFeedback       string // User feedback for re-planning
	EpicPreviousPlan   string // JSON of previous proposed tasks for re-planning context
	APIURL             string // For epic/setup agent to call back to server

	// Setup fields
	SetupRepoID string

	// Common fields
	GitHubToken                string
	GitHubRepo                 string
	AnthropicAPIKey            string
	AnthropicBaseURL           string // Custom base URL for Anthropic API (e.g. for proxies or self-hosted endpoints)
	ClaudeCodeOAuthToken       string // OAuth token (subscription-based, alternative to API key)
	ClaudeModel                string
	DryRun                     bool
	GitHubInsecureSkipVerify   bool
	StripAnthropicBetaHeaders  bool   // Pass through to agent container to run a local header-stripping proxy

	// Repo setup data (injected into agent prompts)
	RepoSummary      string
	RepoExpectations string
	RepoTechStack    string
}

// LogCallback is called for each log line from the container
type LogCallback func(line string)

// RunAgent runs the agent container and streams logs via the callback in real-time.
// The callback is called from a separate goroutine as logs arrive.
func (d *DockerRunner) RunAgent(ctx context.Context, cfg AgentConfig, onLog LogCallback) RunResult {
	workType := cfg.WorkType
	if workType == "" {
		workType = "task"
	}

	// Create container with all required environment variables
	env := []string{
		"WORK_TYPE=" + workType,
		"GITHUB_TOKEN=" + cfg.GitHubToken,
		"GITHUB_REPO=" + cfg.GitHubRepo,
	}

	if cfg.GitHubInsecureSkipVerify {
		env = append(env, "GITHUB_INSECURE_SKIP_VERIFY=true")
	}

	// Pass whichever auth method is configured (OAuth token takes precedence)
	if cfg.ClaudeCodeOAuthToken != "" {
		env = append(env, "CLAUDE_CODE_OAUTH_TOKEN="+cfg.ClaudeCodeOAuthToken)
	} else {
		env = append(env, "ANTHROPIC_API_KEY="+cfg.AnthropicAPIKey)
	}

	if cfg.AnthropicBaseURL != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+cfg.AnthropicBaseURL)
	}

	if cfg.StripAnthropicBetaHeaders {
		env = append(env, "STRIP_ANTHROPIC_BETA_HEADERS=true")
	}

	// Repo setup data — injected into agent prompts for context
	if cfg.RepoSummary != "" {
		env = append(env, "REPO_SUMMARY="+cfg.RepoSummary)
	}
	if cfg.RepoExpectations != "" {
		env = append(env, "REPO_EXPECTATIONS="+cfg.RepoExpectations)
	}
	if cfg.RepoTechStack != "" {
		env = append(env, "REPO_TECH_STACK="+cfg.RepoTechStack)
	}

	switch workType {
	case workTypeSetup:
		// Setup-specific env vars
		env = append(env,
			"REPO_ID="+cfg.SetupRepoID,
			"TASK_ID="+cfg.TaskID,
			"API_URL="+cfg.APIURL,
			"CLAUDE_MODEL="+cfg.ClaudeModel,
		)
	case workTypeEpic:
		// Epic-specific env vars
		env = append(env,
			"EPIC_ID="+cfg.EpicID,
			"EPIC_TITLE="+cfg.EpicTitle,
			"EPIC_DESCRIPTION="+cfg.EpicDescription,
			"EPIC_PLANNING_PROMPT="+cfg.EpicPlanningPrompt,
			"API_URL="+cfg.APIURL,
			"CLAUDE_MODEL="+cfg.ClaudeModel,
		)
		if cfg.EpicFeedback != "" {
			env = append(env, "EPIC_FEEDBACK="+cfg.EpicFeedback)
		}
		if cfg.EpicPreviousPlan != "" {
			env = append(env, "EPIC_PREVIOUS_PLAN="+cfg.EpicPreviousPlan)
		}
	default:
		// Task-specific env vars
		env = append(env,
			"TASK_ID="+cfg.TaskID,
			"TASK_TITLE="+cfg.TaskTitle,
			"TASK_DESCRIPTION="+cfg.TaskDescription,
			"CLAUDE_MODEL="+cfg.ClaudeModel,
		)
		if cfg.DryRun {
			env = append(env, "DRY_RUN=true")
		}
		if cfg.SkipPR {
			env = append(env, "SKIP_PR=true")
		}
		if cfg.DraftPR {
			env = append(env, "DRAFT_PR=true")
		}
		if cfg.Attempt > 1 {
			env = append(env,
				fmt.Sprintf("ATTEMPT=%d", cfg.Attempt),
				"RETRY_REASON="+cfg.RetryReason,
			)
			if cfg.RetryContext != "" {
				env = append(env, "RETRY_CONTEXT="+cfg.RetryContext)
			}
			if cfg.PreviousStatus != "" {
				env = append(env, "PREVIOUS_STATUS="+cfg.PreviousStatus)
			}
		}
		if len(cfg.AcceptanceCriteria) > 0 {
			var ac string
			for i, c := range cfg.AcceptanceCriteria {
				if i > 0 {
					ac += "\n"
				}
				ac += fmt.Sprintf("%d. %s", i+1, c)
			}
			env = append(env, "ACCEPTANCE_CRITERIA="+ac)
		}
	}

	// Container name
	containerName := "verve-"
	switch workType {
	case workTypeSetup:
		containerName += "setup-" + cfg.SetupRepoID
	case workTypeEpic:
		containerName += "epic-" + cfg.EpicID
	default:
		containerName += "task-" + cfg.TaskID
	}

	hostConfig := &container.HostConfig{
		AutoRemove: false, // We'll remove it manually after getting logs
	}

	// Epic planning and setup scan containers need to call back to the API server.
	// Three deployment scenarios are handled:
	// 1. Docker Compose: worker is in Docker, attach container to
	//    the same network so Docker DNS resolves service names.
	// 2. Local dev: worker on host with API_URL=localhost — rewrite to
	//    host.docker.internal so the container can reach the host.
	// 3. Distributed: worker on host with API_URL pointing to a remote
	//    server (e.g. EC2) — no rewrite needed, the container reaches
	//    the remote server over the default bridge network.
	var networkConfig *network.NetworkingConfig
	if workType == workTypeEpic || workType == workTypeSetup {
		if netName := d.detectNetwork(ctx); netName != "" {
			d.logger.Info("attaching epic container to worker network", "container.network", netName)
			networkConfig = &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					netName: {},
				},
			}
		} else {
			// Worker is running on the host (not in Docker). The API URL
			// may use localhost/127.0.0.1 which won't resolve inside the
			// container. Rewrite to host.docker.internal and add the
			// extra host mapping so Docker resolves it to the host.
			rewritten := rewriteLocalhostURL(cfg.APIURL)
			if rewritten != cfg.APIURL {
				d.logger.Info("rewriting api url for container access", "api.original_url", cfg.APIURL, "api.rewritten_url", rewritten)
				// Update the API_URL in the env slice
				for i, e := range env {
					if strings.HasPrefix(e, "API_URL=") {
						env[i] = "API_URL=" + rewritten
						break
					}
				}
				hostConfig.ExtraHosts = append(hostConfig.ExtraHosts, "host.docker.internal:host-gateway")
			}
		}
	}

	resp, err := d.client.ContainerCreate(ctx,
		&container.Config{
			Image: d.agentImage,
			Env:   env,
		},
		hostConfig,
		networkConfig, nil,
		containerName,
	)
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to create container %s: %w", containerName, err)}
	}
	containerID := resp.ID
	d.logger.Info("container created", "container.name", containerName, "container.id", containerID[:12])

	// Ensure cleanup
	defer func() {
		// Remove container
		if err := d.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true}); err != nil {
			d.logger.Warn("failed to remove container", "container.name", containerName, "error", err)
		}
	}()

	// Start container
	if err := d.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return RunResult{Error: fmt.Errorf("failed to start container %s: %w", containerName, err)}
	}
	d.logger.Info("container started", "container.name", containerName)

	// Attach to logs with Follow=true for real-time streaming
	logReader, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true, // Stream logs in real-time
		Timestamps: false,
	})
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to attach logs: %w", err)}
	}

	// Stream logs in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = logReader.Close() }()
		d.streamLogs(logReader, onLog)
	}()

	// Wait for container to finish
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case err := <-errCh:
		return RunResult{Error: fmt.Errorf("error waiting for container: %w", err)}
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-ctx.Done():
		// Context was cancelled (task stopped by user or shutdown).
		// Explicitly stop the container so the agent process is killed immediately
		// rather than waiting for the deferred ContainerRemove.
		d.logger.Info("stopping container due to context cancellation", "container.name", containerName)
		stopTimeout := 5 // seconds
		if err := d.client.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
			d.logger.Warn("failed to stop container, will force-remove", "container.name", containerName, "error", err)
		}
		// Wait for log streaming goroutine to finish (it will end once the container stops)
		wg.Wait()
		return RunResult{Error: ctx.Err()}
	}

	d.logger.Info("container exited", "container.name", containerName, "container.exit_code", exitCode)

	// Wait for log streaming to complete
	wg.Wait()

	return RunResult{
		Success:  exitCode == 0,
		ExitCode: int(exitCode),
	}
}

// streamLogs reads from the Docker multiplexed log stream and calls the callback for each line
func (d *DockerRunner) streamLogs(reader io.Reader, onLog LogCallback) {
	// Create a pipe to demultiplex Docker's stream format
	stdoutPipeR, stdoutPipeW := io.Pipe()
	stderrPipeR, stderrPipeW := io.Pipe()

	// Demultiplex in a goroutine
	go func() {
		defer func() { _ = stdoutPipeW.Close() }()
		defer func() { _ = stderrPipeW.Close() }()
		_, err := stdcopy.StdCopy(stdoutPipeW, stderrPipeW, reader)
		if err != nil && !errors.Is(err, io.EOF) {
			d.logger.Warn("error demultiplexing logs", "error", err)
		}
	}()

	// Read stdout and stderr concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		readLines(stdoutPipeR, func(line string) {
			onLog(line)
		})
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		readLines(stderrPipeR, func(line string) {
			onLog("[stderr] " + line)
		})
	}()

	wg.Wait()
}

// readLines reads lines from a reader and calls the callback for each line
func readLines(reader io.Reader, onLine func(string)) {
	buf := make([]byte, 4096)
	var lineBuf []byte

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Process the data
			data := buf[:n]
			for _, b := range data {
				if b == '\n' {
					if len(lineBuf) > 0 {
						onLine(string(lineBuf))
						lineBuf = lineBuf[:0]
					}
				} else {
					lineBuf = append(lineBuf, b)
				}
			}
		}
		if err != nil {
			// Flush any remaining data
			if len(lineBuf) > 0 {
				onLine(string(lineBuf))
			}
			break
		}
	}
}

// rewriteLocalhostURL replaces localhost or 127.0.0.1 in a URL with
// host.docker.internal so that containers can reach the host machine.
func rewriteLocalhostURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		port := u.Port()
		if port != "" {
			u.Host = "host.docker.internal:" + port
		} else {
			u.Host = "host.docker.internal"
		}
		return u.String()
	}
	return rawURL
}

// detectNetwork returns the name of a user-defined Docker network the worker
// is connected to, or "" if none is found (e.g. the worker runs on the host).
// This is used to attach epic planning containers to the same network so they
// can resolve service names like "server" via Docker DNS.
func (d *DockerRunner) detectNetwork(ctx context.Context) string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}

	info, err := d.client.ContainerInspect(ctx, hostname)
	if err != nil {
		// Not running inside Docker, or hostname doesn't match a container ID
		return ""
	}

	if info.NetworkSettings == nil {
		return ""
	}

	// Return the first non-default network (Compose creates user-defined networks)
	for name := range info.NetworkSettings.Networks {
		if name != "bridge" && name != "host" && name != "none" {
			return name
		}
	}

	return ""
}
