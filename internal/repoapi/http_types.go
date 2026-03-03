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
