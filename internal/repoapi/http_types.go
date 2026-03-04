package repoapi

import (
	"strings"

	"github.com/cohesivestack/valgo"

	"github.com/joshjon/verve/internal/repo"
)

// AddRepoRequest is the request body for adding a repo.
type AddRepoRequest struct {
	FullName string `json:"full_name"`
}

func (r AddRepoRequest) Validate() error {
	return valgo.Is(
		valgo.String(r.FullName, "full_name").Not().Blank().Passing(
			func(s string) bool { return strings.Contains(s, "/") },
			"Must be in owner/repo format",
		),
	).ToError()
}

// RemoveRepoRequest captures the repo_id path parameter.
type RemoveRepoRequest struct {
	RepoID string `param:"repo_id" json:"-"`
}

func (r RemoveRepoRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// RepoIDRequest captures just the repo_id path parameter.
type RepoIDRequest struct {
	RepoID string `param:"repo_id" json:"-"`
}

func (r RepoIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// UpdateSetupRequest is the request body for updating repo setup configuration.
// All fields are optional — only provided fields are updated.
type UpdateSetupRequest struct {
	RepoID       string   `param:"repo_id" json:"-"`
	Summary      *string  `json:"summary,omitempty"`
	Expectations *string  `json:"expectations,omitempty"`
	TechStack    *[]string `json:"tech_stack,omitempty"`
	MarkReady    bool     `json:"mark_ready"`
}

func (r UpdateSetupRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// SubmitSetupRequest is the request body for submitting repo setup configuration
// to the AI agent for review. The agent will flesh out the user's input.
type SubmitSetupRequest struct {
	RepoID       string    `param:"repo_id" json:"-"`
	Summary      *string   `json:"summary,omitempty"`
	Expectations *string   `json:"expectations,omitempty"`
	TechStack    *[]string `json:"tech_stack,omitempty"`
}

func (r SubmitSetupRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}
