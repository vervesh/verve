package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no sensitive data",
			input: "building project...",
			want:  "building project...",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "bearer token in curl",
			input: `curl -H Authorization:Bearer sk-abc123def456 https://api.example.com`,
			want:  `curl -H Authorization:Bearer [REDACTED] https://api.example.com`,
		},
		{
			name:  "bearer token in curl with quotes",
			input: `curl -H "Authorization: Bearer sk-abc123def456" https://api.example.com`,
			want:  `curl -H "Authorization: Bearer [REDACTED] https://api.example.com`,
		},
		{
			name:  "basic auth header",
			input: `Authorization: Basic dXNlcjpwYXNz`,
			want:  `Authorization: Basic [REDACTED]`,
		},
		{
			name:  "x-api-key header",
			input: `X-Api-Key: my-secret-key-123`,
			want:  `X-Api-Key: [REDACTED]`,
		},
		{
			name:  "github personal access token",
			input: `git clone https://ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn@github.com/org/repo`,
			want:  `git clone https://[REDACTED]@github.com/org/repo`,
		},
		{
			name:  "github pat token",
			input: `token=github_pat_ABCDEFGHIJKLMNOPQRSTUVWXYZab`,
			want:  `token=[REDACTED]`,
		},
		{
			name:  "openai api key",
			input: `export OPENAI_API_KEY=sk-proj1234567890abcdefghij`,
			want:  `export OPENAI_API_KEY=[REDACTED]`,
		},
		{
			name:  "anthropic api key",
			input: `ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnopqrst`,
			want:  `ANTHROPIC_API_KEY=[REDACTED]`,
		},
		{
			name:  "generic secret in env var",
			input: `SECRET=mysupersecretvalue123`,
			want:  `SECRET=[REDACTED]`,
		},
		{
			name:  "password in config",
			input: `password: hunter2`,
			want:  `password: [REDACTED]`,
		},
		{
			name:  "token assignment",
			input: `token=abc123xyz`,
			want:  `token=[REDACTED]`,
		},
		{
			name:  "aws access key",
			input: `Found credentials AKIAIOSFODNN7EXAMPLE in config`,
			want:  `Found credentials [REDACTED] in config`,
		},
		{
			name:  "aws secret key",
			input: `aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`,
			want:  `aws_secret_access_key=[REDACTED]`,
		},
		{
			name:  "slack bot token",
			input: `Using token ` + "xoxb-" + `not-a-real-slack-token-value`,
			want:  `Using token [REDACTED]`,
		},
		{
			name:  "private key header",
			input: `-----BEGIN RSA PRIVATE KEY-----`,
			want:  `[REDACTED]`,
		},
		{
			name:  "private key header non-rsa",
			input: `-----BEGIN PRIVATE KEY-----`,
			want:  `[REDACTED]`,
		},
		{
			name:  "postgres connection string",
			input: `postgres://user:s3cret@localhost:5432/mydb`,
			want:  `postgres://user:[REDACTED]@localhost:5432/mydb`,
		},
		{
			name:  "mysql connection string",
			input: `mysql://admin:password123@db.example.com/app`,
			want:  `mysql://admin:[REDACTED]@db.example.com/app`,
		},
		{
			name:  "redis connection string",
			input: `redis://default:mypassword@redis.example.com:6379`,
			want:  `redis://default:[REDACTED]@redis.example.com:6379`,
		},
		{
			name:  "api_key in query param style",
			input: `api_key=supersecretkey123`,
			want:  `api_key=[REDACTED]`,
		},
		{
			name:  "credentials in key=value",
			input: `credentials=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0`,
			want:  `credentials=[REDACTED]`,
		},
		{
			name:  "preserves non-sensitive content",
			input: `Compiling main.go... 42 files processed in 3.2s`,
			want:  `Compiling main.go... 42 files processed in 3.2s`,
		},
		{
			name:  "case insensitive matching",
			input: `PASSWORD=hunter2`,
			want:  `PASSWORD=[REDACTED]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Line(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLines(t *testing.T) {
	input := []string{
		"Starting build...",
		"curl -H \"Authorization: Bearer sk-secret123\" https://api.example.com",
		"Build complete.",
	}

	got := Lines(input)

	assert.Equal(t, "Starting build...", got[0])
	assert.Contains(t, got[1], "[REDACTED]")
	assert.NotContains(t, got[1], "sk-secret123")
	assert.Equal(t, "Build complete.", got[2])
}

func TestLines_DoesNotModifyOriginal(t *testing.T) {
	input := []string{
		"token=secret123",
	}
	original := input[0]

	_ = Lines(input)

	assert.Equal(t, original, input[0], "Lines should not modify the original slice")
}
