package epicapi

import (
	"strconv"

	"github.com/cohesivestack/valgo"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/repo"
)

// --- Request types ---

// CreateEpicRequest is the request body for creating an epic.
type CreateEpicRequest struct {
	RepoID         string `param:"repo_id" json:"-"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	PlanningPrompt string `json:"planning_prompt,omitempty"`
	Model          string `json:"model,omitempty"`
}

func (r CreateEpicRequest) Validate() error {
	return valgo.
		In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).
		Is(valgo.String(r.Title, "title").Not().Blank().MaxLength(200)).
		ToError()
}

// RepoIDRequest captures the :repo_id path parameter.
type RepoIDRequest struct {
	RepoID string `param:"repo_id" json:"-"`
}

func (r RepoIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// EpicByNumberRequest captures the :repo_id and :number path parameters for looking up an epic by number.
type EpicByNumberRequest struct {
	RepoID string `param:"repo_id" json:"-"`
	Number string `param:"number" json:"-"`
}

func (r EpicByNumberRequest) Validate() error {
	v := valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id")))
	n, err := strconv.Atoi(r.Number)
	if err != nil || n <= 0 {
		v = v.AddErrorMessage("number", "must be a positive integer")
	}
	return v.ToError()
}

// EpicIDRequest captures the :id path parameter.
type EpicIDRequest struct {
	ID string `param:"id" json:"-"`
}

func (r EpicIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).ToError()
}

// StartPlanningRequest is the request body for starting a planning session.
type StartPlanningRequest struct {
	ID     string `param:"id" json:"-"`
	Prompt string `json:"prompt"`
}

func (r StartPlanningRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).
		Is(valgo.String(r.Prompt, "prompt").Not().Blank()).
		ToError()
}

// UpdateProposedTasksRequest is the request body for updating proposed tasks.
type UpdateProposedTasksRequest struct {
	ID    string              `param:"id" json:"-"`
	Tasks []epic.ProposedTask `json:"tasks"`
}

func (r UpdateProposedTasksRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).ToError()
}

// SessionMessageRequest is the request body for sending a message in a planning session.
type SessionMessageRequest struct {
	ID      string `param:"id" json:"-"`
	Message string `json:"message"`
}

func (r SessionMessageRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).
		Is(valgo.String(r.Message, "message").Not().Blank()).
		ToError()
}

// ConfirmEpicRequest is the request body for confirming an epic.
type ConfirmEpicRequest struct {
	ID       string `param:"id" json:"-"`
	NotReady bool   `json:"not_ready,omitempty"`
}

func (r ConfirmEpicRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).ToError()
}
