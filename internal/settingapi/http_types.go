package settingapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/joshjon/verve/internal/githubtoken"
)

// SaveGitHubTokenRequest is the request body for saving a GitHub token.
type SaveGitHubTokenRequest struct {
	Token string `json:"token"`
}

func (r SaveGitHubTokenRequest) Validate() error {
	return valgo.Is(
		valgo.String(r.Token, "token").Not().Blank().Passing(
			githubtoken.IsValidTokenPrefix,
			"Must be a GitHub personal access token starting with ghp_ or github_pat_",
		),
	).ToError()
}

// GitHubTokenStatusResponse indicates whether a GitHub token is configured.
type GitHubTokenStatusResponse struct {
	Configured  bool `json:"configured"`
	FineGrained bool `json:"fine_grained,omitempty"`
}

// DefaultModelRequest is the request body for setting the default model.
type DefaultModelRequest struct {
	Model string `json:"model"`
}

func (r DefaultModelRequest) Validate() error {
	return valgo.Is(valgo.String(r.Model, "model").Not().Blank()).ToError()
}

// DefaultModelResponse is the response for getting the default model.
type DefaultModelResponse struct {
	Model      string `json:"model"`
	Configured bool   `json:"configured"`
}
