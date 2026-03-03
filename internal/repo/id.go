package repo

import (
	"github.com/cohesivestack/valgo"
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type repoPrefix struct{}

func (repoPrefix) Prefix() string { return "repo" }

// RepoID is the unique identifier for a Repo.
type RepoID struct {
	typeid.TypeID[repoPrefix]
}

// NewRepoID generates a new unique RepoID.
func NewRepoID() RepoID {
	return id.New[RepoID]()
}

// ParseRepoID parses a string into a RepoID.
func ParseRepoID(s string) (RepoID, error) {
	return id.Parse[RepoID](s)
}

// MustParseRepoID parses a string into a RepoID, panicking on failure.
func MustParseRepoID(s string) RepoID {
	return id.MustParse[RepoID](s)
}

// RepoIDValidator returns a valgo Validator that checks whether the given
// string is a valid RepoID.
func RepoIDValidator(identifier string, nameAndTitle ...string) *valgo.ValidatorString[string] {
	return valgo.String(identifier, nameAndTitle...).
		Not().Blank().
		Passing(func(_ string) bool {
			_, err := ParseRepoID(identifier)
			return err == nil
		}, "Must be a valid repo ID")
}
