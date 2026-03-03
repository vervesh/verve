package metric

import (
	"context"
	"time"

	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

// PlanningEpic represents an epic that is actively being planned by an agent.
type PlanningEpic struct {
	ID        string
	Title     string
	RepoID    string
	Model     string
	ClaimedAt *time.Time
}

// PlanningEpicLister lists epics that are actively being planned.
type PlanningEpicLister interface {
	ListPlanningEpicsForMetrics(ctx context.Context) ([]PlanningEpic, error)
}

// PlanningEpicListerFunc adapts a function to the PlanningEpicLister interface.
type PlanningEpicListerFunc struct {
	fn func(ctx context.Context) ([]PlanningEpic, error)
}

// NewPlanningEpicListerFunc creates a PlanningEpicLister from a function.
func NewPlanningEpicListerFunc(fn func(ctx context.Context) ([]PlanningEpic, error)) *PlanningEpicListerFunc {
	return &PlanningEpicListerFunc{fn: fn}
}

func (f *PlanningEpicListerFunc) ListPlanningEpicsForMetrics(ctx context.Context) ([]PlanningEpic, error) {
	return f.fn(ctx)
}

// TaskLister lists all tasks for metrics computation.
type TaskLister interface {
	ListTasks(ctx context.Context) ([]*task.Task, error)
}

// Metrics provides a snapshot of agent activity and performance.
type Metrics struct {
	// Currently running agents
	RunningAgents int `json:"running_agents"`
	// Tasks pending to be picked up
	PendingTasks int `json:"pending_tasks"`
	// Tasks in review (PR created, awaiting merge)
	ReviewTasks int `json:"review_tasks"`
	// Total tasks (all statuses)
	TotalTasks int `json:"total_tasks"`

	// Completed tasks (merged + closed)
	CompletedTasks int `json:"completed_tasks"`
	// Failed tasks
	FailedTasks int `json:"failed_tasks"`

	// Total cost across all tasks (USD)
	TotalCostUSD float64 `json:"total_cost_usd"`

	// Details about each currently running agent
	ActiveAgents []ActiveAgent `json:"active_agents"`

	// Recent completions (last 10 tasks that reached a terminal state)
	RecentCompletions []CompletedAgent `json:"recent_completions"`

	// Workers actively polling for tasks
	Workers []workertracker.WorkerInfo `json:"workers"`
}

// ActiveAgent describes a single running agent session.
type ActiveAgent struct {
	TaskID     string  `json:"task_id"`
	TaskTitle  string  `json:"task_title"`
	RepoID     string  `json:"repo_id"`
	StartedAt  string  `json:"started_at"`
	RunningFor int64   `json:"running_for_ms"`
	Attempt    int     `json:"attempt"`
	CostUSD    float64 `json:"cost_usd"`
	Model      string  `json:"model,omitempty"`
	EpicID     string  `json:"epic_id,omitempty"`
	IsPlanning bool    `json:"is_planning,omitempty"`
	EpicTitle  string  `json:"epic_title,omitempty"`
}

// CompletedAgent describes a recently completed agent session.
type CompletedAgent struct {
	TaskID     string  `json:"task_id"`
	TaskTitle  string  `json:"task_title"`
	RepoID     string  `json:"repo_id"`
	Status     string  `json:"status"`
	DurationMs *int64  `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"cost_usd"`
	Attempt    int     `json:"attempt"`
	FinishedAt string  `json:"finished_at"`
}

// Compute calculates agent observability metrics from current task data,
// planning epics, and worker info.
func Compute(ctx context.Context, lister TaskLister, epicLister PlanningEpicLister, workers []workertracker.WorkerInfo) (*Metrics, error) {
	tasks, err := lister.ListTasks(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	m := &Metrics{}

	var recentTerminal []*task.Task

	for _, t := range tasks {
		m.TotalTasks++
		m.TotalCostUSD += t.CostUSD

		switch t.Status {
		case task.StatusPending:
			m.PendingTasks++
		case task.StatusRunning:
			m.RunningAgents++
			agent := ActiveAgent{
				TaskID:    t.ID.String(),
				TaskTitle: t.Title,
				RepoID:    t.RepoID,
				Attempt:   t.Attempt,
				CostUSD:   t.CostUSD,
				Model:     t.Model,
				EpicID:    t.EpicID,
			}
			if t.StartedAt != nil {
				agent.StartedAt = t.StartedAt.Format(time.RFC3339)
				agent.RunningFor = now.Sub(*t.StartedAt).Milliseconds()
			}
			m.ActiveAgents = append(m.ActiveAgents, agent)
		case task.StatusReview:
			m.ReviewTasks++
		case task.StatusMerged, task.StatusClosed:
			m.CompletedTasks++
			recentTerminal = append(recentTerminal, t)
		case task.StatusFailed:
			m.FailedTasks++
			recentTerminal = append(recentTerminal, t)
		}
	}

	// Include epics that are actively being planned as active agents.
	if epicLister != nil {
		planningEpics, err := epicLister.ListPlanningEpicsForMetrics(ctx)
		if err == nil {
			for _, ep := range planningEpics {
				m.RunningAgents++
				agent := ActiveAgent{
					TaskID:     ep.ID,
					TaskTitle:  ep.Title,
					RepoID:     ep.RepoID,
					Model:      ep.Model,
					EpicID:     ep.ID,
					IsPlanning: true,
					EpicTitle:  ep.Title,
				}
				if ep.ClaimedAt != nil {
					agent.StartedAt = ep.ClaimedAt.Format(time.RFC3339)
					agent.RunningFor = now.Sub(*ep.ClaimedAt).Milliseconds()
				}
				m.ActiveAgents = append(m.ActiveAgents, agent)
			}
		}
	}

	// Sort terminal tasks by updated_at descending (most recent first) and take top 10.
	if len(recentTerminal) > 0 {
		for i := 0; i < len(recentTerminal); i++ {
			for j := i + 1; j < len(recentTerminal); j++ {
				if recentTerminal[j].UpdatedAt.After(recentTerminal[i].UpdatedAt) {
					recentTerminal[i], recentTerminal[j] = recentTerminal[j], recentTerminal[i]
				}
			}
		}
		limit := 10
		if len(recentTerminal) < limit {
			limit = len(recentTerminal)
		}
		for _, t := range recentTerminal[:limit] {
			t.ComputeDuration()
			c := CompletedAgent{
				TaskID:     t.ID.String(),
				TaskTitle:  t.Title,
				RepoID:     t.RepoID,
				Status:     string(t.Status),
				DurationMs: t.DurationMs,
				CostUSD:    t.CostUSD,
				Attempt:    t.Attempt,
				FinishedAt: t.UpdatedAt.Format(time.RFC3339),
			}
			m.RecentCompletions = append(m.RecentCompletions, c)
		}
	}

	if m.ActiveAgents == nil {
		m.ActiveAgents = []ActiveAgent{}
	}
	if m.RecentCompletions == nil {
		m.RecentCompletions = []CompletedAgent{}
	}
	if workers != nil {
		m.Workers = workers
	} else {
		m.Workers = []workertracker.WorkerInfo{}
	}

	return m, nil
}
