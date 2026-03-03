package task

import (
	"github.com/cohesivestack/valgo"
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type taskPrefix struct{}

func (taskPrefix) Prefix() string { return "tsk" }

// TaskID is the unique identifier for a Task.
type TaskID struct {
	typeid.TypeID[taskPrefix]
}

// NewTaskID generates a new unique TaskID.
func NewTaskID() TaskID {
	return id.New[TaskID]()
}

// ParseTaskID parses a string into a TaskID.
func ParseTaskID(s string) (TaskID, error) {
	return id.Parse[TaskID](s)
}

// MustParseTaskID parses a string into a TaskID, panicking on failure.
func MustParseTaskID(s string) TaskID {
	return id.MustParse[TaskID](s)
}

// TaskIDValidator returns a valgo Validator that checks whether the given
// string is a valid TaskID.
func TaskIDValidator(identifier string, nameAndTitle ...string) *valgo.ValidatorString[string] {
	return valgo.String(identifier, nameAndTitle...).
		Not().Blank().
		Passing(func(_ string) bool {
			_, err := ParseTaskID(identifier)
			return err == nil
		}, "Must be a valid task ID")
}
