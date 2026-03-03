package epic

import (
	"github.com/cohesivestack/valgo"
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type epicPrefix struct{}

func (epicPrefix) Prefix() string { return "epc" }

// EpicID is the unique identifier for an Epic.
type EpicID struct {
	typeid.TypeID[epicPrefix]
}

// NewEpicID generates a new unique EpicID.
func NewEpicID() EpicID {
	return id.New[EpicID]()
}

// ParseEpicID parses a string into an EpicID.
func ParseEpicID(s string) (EpicID, error) {
	return id.Parse[EpicID](s)
}

// MustParseEpicID parses a string into an EpicID, panicking on failure.
func MustParseEpicID(s string) EpicID {
	return id.MustParse[EpicID](s)
}

// EpicIDValidator returns a valgo Validator that checks whether the given
// string is a valid EpicID.
func EpicIDValidator(identifier string, nameAndTitle ...string) *valgo.ValidatorString[string] {
	return valgo.String(identifier, nameAndTitle...).
		Not().Blank().
		Passing(func(_ string) bool {
			_, err := ParseEpicID(identifier)
			return err == nil
		}, "Must be a valid epic ID")
}
