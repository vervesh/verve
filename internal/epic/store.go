package epic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/joshjon/kit/log"
)

// TaskCreator creates tasks in the task system when an epic is confirmed.
type TaskCreator interface {
	CreateTaskFromEpic(ctx context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool, model string) (string, error)
}

// TaskStatusReader reads task statuses for epic completion checking.
type TaskStatusReader interface {
	ReadTaskStatus(ctx context.Context, taskID string) (string, error)
}

// Store wraps a Repository and adds application-level concerns for epics.
type Store struct {
	repo             Repository
	taskCreator      TaskCreator
	taskStatusReader TaskStatusReader
	logger           log.Logger

	// Pending epic notification (same pattern as task.Store)
	pendingMu sync.Mutex
	pendingCh chan struct{}

	// Per-epic feedback notification channels
	feedbackMu    sync.Mutex
	feedbackChans map[string]chan struct{}
}

// NewStore creates a new Store backed by the given Repository.
func NewStore(repo Repository, taskCreator TaskCreator, logger log.Logger) *Store {
	return &Store{
		repo:          repo,
		taskCreator:   taskCreator,
		logger:        logger.With("component", "epic_store"),
		pendingCh:     make(chan struct{}, 1),
		feedbackChans: make(map[string]chan struct{}),
	}
}

// SetTaskStatusReader sets the TaskStatusReader used for epic completion checks.
// This is set after construction to avoid circular dependencies.
func (s *Store) SetTaskStatusReader(reader TaskStatusReader) {
	s.taskStatusReader = reader
}

// WaitForPending returns a channel that signals when a planning epic might be available.
func (s *Store) WaitForPending() <-chan struct{} {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return s.pendingCh
}

func (s *Store) notifyPending() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	select {
	case s.pendingCh <- struct{}{}:
	default:
	}
}

// getFeedbackChan returns (or creates) the feedback notification channel for an epic.
func (s *Store) getFeedbackChan(id string) chan struct{} {
	s.feedbackMu.Lock()
	defer s.feedbackMu.Unlock()
	ch, ok := s.feedbackChans[id]
	if !ok {
		ch = make(chan struct{}, 1)
		s.feedbackChans[id] = ch
	}
	return ch
}

func (s *Store) notifyFeedback(id string) {
	s.feedbackMu.Lock()
	ch, ok := s.feedbackChans[id]
	s.feedbackMu.Unlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// WaitForFeedback returns a channel that signals when feedback is available for an epic.
func (s *Store) WaitForFeedback(id string) <-chan struct{} {
	return s.getFeedbackChan(id)
}

// CleanupFeedbackChan removes the feedback channel for an epic (call when agent exits).
func (s *Store) CleanupFeedbackChan(id string) {
	s.feedbackMu.Lock()
	defer s.feedbackMu.Unlock()
	delete(s.feedbackChans, id)
}

// CreateEpic creates a new epic in planning status and notifies pending.
func (s *Store) CreateEpic(ctx context.Context, epic *Epic) error {
	if err := s.repo.CreateEpic(ctx, epic); err != nil {
		return err
	}
	s.notifyPending()
	return nil
}

// ReadEpic reads an epic by ID.
func (s *Store) ReadEpic(ctx context.Context, id EpicID) (*Epic, error) {
	return s.repo.ReadEpic(ctx, id)
}

// ListEpics returns all epics.
func (s *Store) ListEpics(ctx context.Context) ([]*Epic, error) {
	return s.repo.ListEpics(ctx)
}

// ListEpicsByRepo returns all epics for a given repo.
func (s *Store) ListEpicsByRepo(ctx context.Context, repoID string) ([]*Epic, error) {
	return s.repo.ListEpicsByRepo(ctx, repoID)
}

// ClaimPendingEpic finds an unclaimed planning epic and claims it atomically.
func (s *Store) ClaimPendingEpic(ctx context.Context) (*Epic, error) {
	epics, err := s.repo.ListPlanningEpics(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range epics {
		ok, err := s.repo.ClaimEpic(ctx, e.ID)
		if err != nil {
			continue
		}
		if !ok {
			continue // Already claimed by another worker
		}
		// Re-read to get updated claimed_at
		return s.repo.ReadEpic(ctx, e.ID)
	}
	return nil, nil
}

// EpicHeartbeat updates the heartbeat timestamp for a claimed epic.
func (s *Store) EpicHeartbeat(ctx context.Context, id EpicID) error {
	return s.repo.EpicHeartbeat(ctx, id)
}

// SetFeedback stores user feedback and notifies the waiting agent.
func (s *Store) SetFeedback(ctx context.Context, id EpicID, feedback string, feedbackType FeedbackType) error {
	if err := s.repo.SetEpicFeedback(ctx, id, feedback, string(feedbackType)); err != nil {
		return err
	}
	// If it's a message, transition back to planning status
	if feedbackType == FeedbackMessage {
		if err := s.repo.UpdateEpicStatus(ctx, id, StatusPlanning); err != nil {
			return err
		}
	}
	s.notifyFeedback(id.String())
	return nil
}

// PollFeedback reads and clears pending feedback for an epic.
// Returns nil values if no feedback is pending.
func (s *Store) PollFeedback(ctx context.Context, id EpicID) (*string, *string, error) {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if e.Feedback == nil && e.FeedbackType == nil {
		return nil, nil, nil
	}
	feedback := e.Feedback
	feedbackType := e.FeedbackType
	if err := s.repo.ClearEpicFeedback(ctx, id); err != nil {
		return nil, nil, err
	}
	return feedback, feedbackType, nil
}

// UpdateProposedTasks updates the proposed tasks and transitions to draft status.
func (s *Store) UpdateProposedTasks(ctx context.Context, id EpicID, tasks []ProposedTask) error {
	if err := s.repo.UpdateProposedTasks(ctx, id, tasks); err != nil {
		return err
	}
	return s.repo.UpdateEpicStatus(ctx, id, StatusDraft)
}

// AppendSessionLog appends messages to the planning session log.
func (s *Store) AppendSessionLog(ctx context.Context, id EpicID, lines []string) error {
	return s.repo.AppendSessionLog(ctx, id, lines)
}

// StartPlanning transitions an epic back to planning status and notifies pending.
func (s *Store) StartPlanning(ctx context.Context, id EpicID, prompt string) error {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft && e.Status != StatusReady {
		return fmt.Errorf("epic must be in draft or ready status to start planning")
	}
	e.PlanningPrompt = prompt
	e.Status = StatusPlanning
	e.UpdatedAt = time.Now()
	if err := s.repo.UpdateEpic(ctx, e); err != nil {
		return err
	}
	s.notifyPending()
	return nil
}

// ConfirmEpic creates real tasks from proposed tasks and activates the epic.
// Also signals the agent to exit via feedback.
func (s *Store) ConfirmEpic(ctx context.Context, id EpicID, notReady bool) error {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft && e.Status != StatusReady {
		return fmt.Errorf("epic must be in draft or ready status to confirm")
	}
	if len(e.ProposedTasks) == 0 {
		return fmt.Errorf("epic has no proposed tasks to confirm")
	}

	// Map temp IDs to real task IDs
	tempToReal := make(map[string]string)
	taskIDs := make([]string, 0, len(e.ProposedTasks))

	// Create tasks in dependency order
	for _, pt := range e.ProposedTasks {
		var realDeps []string
		for _, depTempID := range pt.DependsOnTempIDs {
			if realID, ok := tempToReal[depTempID]; ok {
				realDeps = append(realDeps, realID)
			}
		}

		taskID, err := s.taskCreator.CreateTaskFromEpic(
			ctx,
			e.RepoID,
			pt.Title,
			pt.Description,
			realDeps,
			pt.AcceptanceCriteria,
			id.String(),
			!notReady,
			e.Model,
		)
		if err != nil {
			return fmt.Errorf("create task %q: %w", pt.Title, err)
		}

		tempToReal[pt.TempID] = taskID
		taskIDs = append(taskIDs, taskID)
	}

	// Store task IDs and update status
	if err := s.repo.SetTaskIDs(ctx, id, taskIDs); err != nil {
		return err
	}

	status := StatusActive
	if notReady {
		status = StatusReady
	}
	e.NotReady = notReady
	if err := s.repo.UpdateEpicStatus(ctx, id, status); err != nil {
		return err
	}

	// Signal agent to exit
	_ = s.repo.SetEpicFeedback(ctx, id, "", string(FeedbackConfirmed))
	s.notifyFeedback(id.String())
	return nil
}

// CloseEpic closes an epic and signals the agent to exit.
func (s *Store) CloseEpic(ctx context.Context, id EpicID) error {
	if err := s.repo.UpdateEpicStatus(ctx, id, StatusClosed); err != nil {
		return err
	}
	// Signal agent to exit
	_ = s.repo.SetEpicFeedback(ctx, id, "", string(FeedbackClosed))
	s.notifyFeedback(id.String())
	return nil
}

// DeleteEpic deletes an epic. If the epic is being planned, the agent is
// signalled to exit. Callers are responsible for deleting child tasks before
// calling this method to avoid FK violations.
func (s *Store) DeleteEpic(ctx context.Context, id EpicID) error {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}

	// Signal agent to exit if epic is in planning or draft state with a claim
	if e.Status == StatusPlanning || e.Status == StatusDraft {
		_ = s.repo.SetEpicFeedback(ctx, id, "", string(FeedbackClosed))
		s.notifyFeedback(id.String())
	}

	return s.repo.DeleteEpic(ctx, id)
}

// ReleaseEpicClaim releases a worker's claim on an epic, making it available again.
func (s *Store) ReleaseEpicClaim(ctx context.Context, id EpicID) error {
	if err := s.repo.ReleaseEpicClaim(ctx, id); err != nil {
		return err
	}
	s.notifyPending()
	return nil
}

// TimeoutStaleEpics releases claimed epics whose heartbeat has expired.
func (s *Store) TimeoutStaleEpics(ctx context.Context, timeout time.Duration) (int, error) {
	threshold := time.Now().Add(-timeout)
	epics, err := s.repo.ListStaleEpics(ctx, threshold)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range epics {
		_ = s.repo.AppendSessionLog(ctx, e.ID, []string{"system: Planning session timed out due to inactivity."})
		if err := s.repo.ReleaseEpicClaim(ctx, e.ID); err != nil {
			continue
		}
		count++
		s.notifyPending()
	}
	return count, nil
}

// RemoveTaskAndCheck removes a task ID from an epic's task_ids list and
// checks if the epic should be marked as completed.
func (s *Store) RemoveTaskAndCheck(ctx context.Context, id EpicID, taskID string) error {
	if err := s.repo.RemoveTaskID(ctx, id, taskID); err != nil {
		return err
	}
	return s.CheckAndCompleteEpic(ctx, id)
}

// CheckAndCompleteEpic checks whether all tasks in an active epic have reached
// a terminal state (merged or closed). If so, the epic is transitioned to completed.
// Tasks in failed status prevent completion — the epic stays active.
func (s *Store) CheckAndCompleteEpic(ctx context.Context, id EpicID) error {
	if s.taskStatusReader == nil {
		return nil
	}

	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}

	// Only check active epics
	if e.Status != StatusActive {
		return nil
	}

	// An epic with no tasks shouldn't auto-complete
	if len(e.TaskIDs) == 0 {
		return nil
	}

	for _, taskID := range e.TaskIDs {
		status, err := s.taskStatusReader.ReadTaskStatus(ctx, taskID)
		if err != nil {
			// Task may have been deleted without updating epic task_ids;
			// treat missing tasks as not blocking completion.
			continue
		}
		switch status {
		case "merged", "closed":
			// Terminal success — doesn't block
			continue
		case "failed":
			// Failed tasks prevent completion
			return nil
		default:
			// Task is still in progress (pending, running, review)
			return nil
		}
	}

	// All tasks are in terminal success state — complete the epic
	s.logger.Info("all tasks completed, marking epic as completed", "epic.id", id.String())
	return s.repo.UpdateEpicStatus(ctx, id, StatusCompleted)
}

// ListPlanningEpicsForMetrics returns epics that are actively being planned
// by a worker agent (claimed epics in planning or draft status). This is used
// by the task metrics to include planning agents in the active agents count.
func (s *Store) ListPlanningEpicsForMetrics(ctx context.Context) ([]PlanningEpicForMetrics, error) {
	epics, err := s.repo.ListEpics(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]PlanningEpicForMetrics, 0, len(epics))
	for _, e := range epics {
		// Only include epics that are actively claimed by a worker
		if e.ClaimedAt == nil {
			continue
		}
		// Only planning or draft status (draft means agent proposed tasks, still active)
		if e.Status != StatusPlanning && e.Status != StatusDraft {
			continue
		}
		result = append(result, PlanningEpicForMetrics{
			ID:        e.ID.String(),
			Title:     e.Title,
			RepoID:    e.RepoID,
			Model:     e.Model,
			ClaimedAt: e.ClaimedAt,
		})
	}
	return result, nil
}

// PlanningEpicForMetrics is a minimal struct for epic planning metrics.
type PlanningEpicForMetrics struct {
	ID        string
	Title     string
	RepoID    string
	Model     string
	ClaimedAt *time.Time
}

// CheckActiveEpicsCompletion checks all active epics for completion.
// This is intended to be called from a background loop.
func (s *Store) CheckActiveEpicsCompletion(ctx context.Context) (int, error) {
	if s.taskStatusReader == nil {
		return 0, nil
	}

	epics, err := s.repo.ListActiveEpics(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, e := range epics {
		if len(e.TaskIDs) == 0 {
			continue
		}
		if err := s.CheckAndCompleteEpic(ctx, e.ID); err != nil {
			s.logger.Error("failed to check epic completion", "epic.id", e.ID.String(), "error", err)
			continue
		}
		// Re-read to check if status changed
		updated, err := s.repo.ReadEpic(ctx, e.ID)
		if err != nil {
			continue
		}
		if updated.Status == StatusCompleted {
			count++
		}
	}
	return count, nil
}
