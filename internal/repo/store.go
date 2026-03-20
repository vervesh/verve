package repo

import (
	"context"
	"fmt"
)

// validSetupTransitions defines which status transitions are allowed.
// The key is the current status, and the values are the allowed target statuses.
var validSetupTransitions = map[string][]string{
	SetupStatusPending:     {SetupStatusScanning, SetupStatusReady},
	SetupStatusScanning:    {SetupStatusNeedsSetup, SetupStatusReady},
	SetupStatusNeedsSetup:  {SetupStatusReady, SetupStatusScanning, SetupStatusConfiguring},
	SetupStatusConfiguring: {SetupStatusNeedsSetup, SetupStatusReady, SetupStatusScanning},
	SetupStatusReady:       {SetupStatusScanning, SetupStatusConfiguring},
}

// Store wraps a Repository and adds application-level concerns.
type Store struct {
	repo Repository
}

// NewStore creates a new Store backed by the given Repository.
func NewStore(repo Repository) *Store {
	return &Store{repo: repo}
}

// CreateRepo creates a new repo.
func (s *Store) CreateRepo(ctx context.Context, repo *Repo) error {
	return s.repo.CreateRepo(ctx, repo)
}

// ReadRepo reads a repo by ID.
func (s *Store) ReadRepo(ctx context.Context, id RepoID) (*Repo, error) {
	return s.repo.ReadRepo(ctx, id)
}

// ReadRepoByFullName reads a repo by its full name (owner/name).
func (s *Store) ReadRepoByFullName(ctx context.Context, fullName string) (*Repo, error) {
	return s.repo.ReadRepoByFullName(ctx, fullName)
}

// ListRepos returns all repos.
func (s *Store) ListRepos(ctx context.Context) ([]*Repo, error) {
	return s.repo.ListRepos(ctx)
}

// DeleteRepo deletes a repo and cascade-deletes all associated epics, tasks,
// and task logs via ON DELETE CASCADE constraints in the database.
func (s *Store) DeleteRepo(ctx context.Context, id RepoID) error {
	return s.repo.DeleteRepo(ctx, id)
}

// UpdateRepoSetupScan updates the scan results for a repo and sets the new
// setup status. The result.SetupStatus must be a valid transition from the
// repo's current status.
func (s *Store) UpdateRepoSetupScan(ctx context.Context, id RepoID, result SetupScanResult) error {
	if !ValidSetupStatus(result.SetupStatus) {
		return fmt.Errorf("invalid setup status %q", result.SetupStatus)
	}
	current, err := s.repo.ReadRepo(ctx, id)
	if err != nil {
		return err
	}
	if !isValidTransition(current.SetupStatus, result.SetupStatus) {
		return fmt.Errorf("invalid setup status transition from %q to %q", current.SetupStatus, result.SetupStatus)
	}
	return s.repo.UpdateRepoSetupScan(ctx, id, result)
}

// UpdateRepoSetupStatus updates the setup status of a repo. It enforces valid
// status transitions.
func (s *Store) UpdateRepoSetupStatus(ctx context.Context, id RepoID, status string) error {
	if !ValidSetupStatus(status) {
		return fmt.Errorf("invalid setup status %q", status)
	}
	current, err := s.repo.ReadRepo(ctx, id)
	if err != nil {
		return err
	}
	if current.SetupStatus == status {
		return nil
	}
	if !isValidTransition(current.SetupStatus, status) {
		return fmt.Errorf("invalid setup status transition from %q to %q", current.SetupStatus, status)
	}
	return s.repo.UpdateRepoSetupStatus(ctx, id, status)
}

// UpdateRepoExpectations updates the expectations text and optionally marks
// setup as completed.
func (s *Store) UpdateRepoExpectations(ctx context.Context, id RepoID, update ExpectationsUpdate) error {
	return s.repo.UpdateRepoExpectations(ctx, id, update)
}

// UpdateRepoSummary updates the summary text for a repo.
func (s *Store) UpdateRepoSummary(ctx context.Context, id RepoID, summary string) error {
	return s.repo.UpdateRepoSummary(ctx, id, summary)
}

// UpdateRepoTechStack updates the tech stack list for a repo.
func (s *Store) UpdateRepoTechStack(ctx context.Context, id RepoID, techStack []string) error {
	if techStack == nil {
		techStack = []string{}
	}
	return s.repo.UpdateRepoTechStack(ctx, id, techStack)
}

// ListReposBySetupStatus returns all repos with the given setup status.
func (s *Store) ListReposBySetupStatus(ctx context.Context, status string) ([]*Repo, error) {
	return s.repo.ListReposBySetupStatus(ctx, status)
}

func isValidTransition(from, to string) bool {
	allowed, ok := validSetupTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
