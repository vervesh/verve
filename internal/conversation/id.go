package conversation

import (
	"github.com/cohesivestack/valgo"
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type conversationPrefix struct{}

func (conversationPrefix) Prefix() string { return "cnv" }

// ConversationID is the unique identifier for a Conversation.
type ConversationID struct {
	typeid.TypeID[conversationPrefix]
}

// NewConversationID generates a new unique ConversationID.
func NewConversationID() ConversationID {
	return id.New[ConversationID]()
}

// ParseConversationID parses a string into a ConversationID.
func ParseConversationID(s string) (ConversationID, error) {
	return id.Parse[ConversationID](s)
}

// MustParseConversationID parses a string into a ConversationID, panicking on failure.
func MustParseConversationID(s string) ConversationID {
	return id.MustParse[ConversationID](s)
}

// ConversationIDValidator returns a valgo Validator that checks whether the given
// string is a valid ConversationID.
func ConversationIDValidator(identifier string, nameAndTitle ...string) *valgo.ValidatorString[string] {
	return valgo.String(identifier, nameAndTitle...).
		Not().Blank().
		Passing(func(_ string) bool {
			_, err := ParseConversationID(identifier)
			return err == nil
		}, "Must be a valid conversation ID")
}
