package repo

import (
	"fmt"
	"strings"
	"time"
)

// SetupStatus represents the current state of repository setup.
const (
	SetupStatusPending    = "pending"
	SetupStatusScanning   = "scanning"
	SetupStatusNeedsSetup = "needs_setup"
	SetupStatusReady      = "ready"
)

// Repo represents a GitHub repository added to Verve.
type Repo struct {
	ID               RepoID     `json:"id"`
	Owner            string     `json:"owner"`
	Name             string     `json:"name"`
	FullName         string     `json:"full_name"`
	Summary          string     `json:"summary"`
	TechStack        []string   `json:"tech_stack"`
	SetupStatus      string     `json:"setup_status"`
	HasCode          bool       `json:"has_code"`
	HasCLAUDEMD      bool       `json:"has_claude_md"`
	HasREADME        bool       `json:"has_readme"`
	Expectations     string     `json:"expectations"`
	SetupCompletedAt *time.Time `json:"setup_completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// NewRepo creates a new Repo from a full name (e.g., "owner/repo").
func NewRepo(fullName string) (*Repo, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repo full name %q: expected owner/name", fullName)
	}
	return &Repo{
		ID:          NewRepoID(),
		Owner:       parts[0],
		Name:        parts[1],
		FullName:    fullName,
		TechStack:   []string{},
		SetupStatus: SetupStatusPending,
		CreatedAt:   time.Now(),
	}, nil
}

// ValidSetupStatus returns true if the given status is a valid setup status.
func ValidSetupStatus(s string) bool {
	switch s {
	case SetupStatusPending, SetupStatusScanning, SetupStatusNeedsSetup, SetupStatusReady:
		return true
	}
	return false
}
