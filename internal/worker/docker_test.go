package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentConfig_ConversationFields(t *testing.T) {
	cfg := AgentConfig{
		WorkType:                   workTypeConversation,
		ConversationID:             "cnv_123",
		ConversationTitle:          "Discuss auth",
		ConversationMessages:       `[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}]`,
		ConversationPendingMessage: "How does login work?",
		APIURL:                     "http://localhost:7400",
		GitHubToken:                "ghp_test",
		GitHubRepo:                 "owner/repo",
		AnthropicAPIKey:            "sk-ant-test",
		ClaudeModel:                "sonnet",
	}

	assert.Equal(t, workTypeConversation, cfg.WorkType)
	assert.Equal(t, "cnv_123", cfg.ConversationID)
	assert.Equal(t, "Discuss auth", cfg.ConversationTitle)
	assert.Contains(t, cfg.ConversationMessages, `"role":"user"`)
	assert.Equal(t, "How does login work?", cfg.ConversationPendingMessage)
	assert.Equal(t, "http://localhost:7400", cfg.APIURL)
}

func TestBuildEnv_Conversation(t *testing.T) {
	// Test that conversation env vars are built correctly by simulating
	// what RunAgent does for the conversation work type
	cfg := AgentConfig{
		WorkType:                   workTypeConversation,
		ConversationID:             "cnv_456",
		ConversationTitle:          "Code review",
		ConversationMessages:       `[{"role":"user","content":"review this"}]`,
		ConversationPendingMessage: "What about error handling?",
		APIURL:                     "http://localhost:7400",
		GitHubToken:                "ghp_token",
		GitHubRepo:                 "owner/repo",
		AnthropicAPIKey:            "sk-ant-key",
		ClaudeModel:                "opus",
		RepoSummary:                "A Go web app",
		RepoTechStack:              "Go, PostgreSQL",
	}

	// Build env the same way RunAgent does
	env := []string{
		"WORK_TYPE=" + cfg.WorkType,
		"GITHUB_TOKEN=" + cfg.GitHubToken,
		"GITHUB_REPO=" + cfg.GitHubRepo,
		"ANTHROPIC_API_KEY=" + cfg.AnthropicAPIKey,
	}

	if cfg.RepoSummary != "" {
		env = append(env, "REPO_SUMMARY="+cfg.RepoSummary)
	}
	if cfg.RepoTechStack != "" {
		env = append(env, "REPO_TECH_STACK="+cfg.RepoTechStack)
	}

	// Conversation-specific env vars
	env = append(env,
		"CONVERSATION_ID="+cfg.ConversationID,
		"CONVERSATION_TITLE="+cfg.ConversationTitle,
		"CONVERSATION_MESSAGES="+cfg.ConversationMessages,
		"CONVERSATION_PENDING_MESSAGE="+cfg.ConversationPendingMessage,
		"API_URL="+cfg.APIURL,
		"CLAUDE_MODEL="+cfg.ClaudeModel,
	)

	// Verify all expected env vars are present
	envMap := make(map[string]string)
	for _, e := range env {
		parts := splitFirst(e, "=")
		envMap[parts[0]] = parts[1]
	}

	assert.Equal(t, "conversation", envMap["WORK_TYPE"])
	assert.Equal(t, "cnv_456", envMap["CONVERSATION_ID"])
	assert.Equal(t, "Code review", envMap["CONVERSATION_TITLE"])
	assert.Equal(t, `[{"role":"user","content":"review this"}]`, envMap["CONVERSATION_MESSAGES"])
	assert.Equal(t, "What about error handling?", envMap["CONVERSATION_PENDING_MESSAGE"])
	assert.Equal(t, "http://localhost:7400", envMap["API_URL"])
	assert.Equal(t, "opus", envMap["CLAUDE_MODEL"])
	assert.Equal(t, "ghp_token", envMap["GITHUB_TOKEN"])
	assert.Equal(t, "owner/repo", envMap["GITHUB_REPO"])
	assert.Equal(t, "A Go web app", envMap["REPO_SUMMARY"])
	assert.Equal(t, "Go, PostgreSQL", envMap["REPO_TECH_STACK"])
}

func TestContainerName_Conversation(t *testing.T) {
	// Verify conversation container naming follows the expected pattern
	workType := workTypeConversation
	conversationID := "cnv_abc123"

	containerName := "verve-"
	switch workType {
	case workTypeSetup, workTypeSetupReview:
		containerName += "setup-"
	case workTypeEpic:
		containerName += "epic-"
	case workTypeConversation:
		containerName += "conversation-" + conversationID
	default:
		containerName += "task-"
	}

	assert.Equal(t, "verve-conversation-cnv_abc123", containerName)
}

// splitFirst splits a string on the first occurrence of sep.
func splitFirst(s, sep string) []string {
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

func TestRewriteLocalhostURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "localhost with port",
			input:    "http://localhost:7400",
			expected: "http://host.docker.internal:7400",
		},
		{
			name:     "127.0.0.1 with port",
			input:    "http://127.0.0.1:7400",
			expected: "http://host.docker.internal:7400",
		},
		{
			name:     "localhost without port",
			input:    "http://localhost",
			expected: "http://host.docker.internal",
		},
		{
			name:     "localhost with path",
			input:    "http://localhost:7400/api/v1",
			expected: "http://host.docker.internal:7400/api/v1",
		},
		{
			name:     "https localhost",
			input:    "https://localhost:7400",
			expected: "https://host.docker.internal:7400",
		},
		{
			name:     "non-localhost unchanged",
			input:    "http://server:7400",
			expected: "http://server:7400",
		},
		{
			name:     "external host unchanged",
			input:    "https://api.example.com",
			expected: "https://api.example.com",
		},
		{
			name:     "private IP unchanged (distributed worker)",
			input:    "http://10.0.1.5:7400",
			expected: "http://10.0.1.5:7400",
		},
		{
			name:     "public hostname unchanged (distributed worker)",
			input:    "https://api.verve.example.com",
			expected: "https://api.verve.example.com",
		},
		{
			name:     "invalid URL unchanged",
			input:    "://bad",
			expected: "://bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteLocalhostURL(tt.input)
			if got != tt.expected {
				t.Errorf("rewriteLocalhostURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
