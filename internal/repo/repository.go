package repo

import (
	"context"
	"time"
)

// SetupScanResult holds the results of scanning a repository.
type SetupScanResult struct {
	Summary     string
	TechStack   []string
	HasCode     bool
	HasCLAUDEMD bool
	HasREADME   bool
	SetupStatus string
}

// ExpectationsUpdate holds the data for updating repo expectations.
type ExpectationsUpdate struct {
	Expectations     string
	SetupCompletedAt *time.Time
}

// Repository is the interface for performing CRUD operations on Repos.
type Repository interface {
	CreateRepo(ctx context.Context, repo *Repo) error
	ReadRepo(ctx context.Context, id RepoID) (*Repo, error)
	ReadRepoByFullName(ctx context.Context, fullName string) (*Repo, error)
	ListRepos(ctx context.Context) ([]*Repo, error)
	DeleteRepo(ctx context.Context, id RepoID) error

	UpdateRepoSetupScan(ctx context.Context, id RepoID, result SetupScanResult) error
	UpdateRepoSetupStatus(ctx context.Context, id RepoID, status string) error
	UpdateRepoExpectations(ctx context.Context, id RepoID, update ExpectationsUpdate) error
	UpdateRepoSummary(ctx context.Context, id RepoID, summary string) error
	UpdateRepoTechStack(ctx context.Context, id RepoID, techStack []string) error
	ListReposBySetupStatus(ctx context.Context, status string) ([]*Repo, error)
}
