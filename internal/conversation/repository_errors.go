package conversation

import "github.com/joshjon/kit/errtag"

// ErrTagConversationNotFound indicates a conversation was not found.
type ErrTagConversationNotFound struct{ errtag.NotFound }

func (ErrTagConversationNotFound) Msg() string { return "Conversation not found" }

func (e ErrTagConversationNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

// ErrTagConversationConflict indicates a conversation conflict (e.g. duplicate ID).
type ErrTagConversationConflict struct{ errtag.Conflict }

func (ErrTagConversationConflict) Msg() string { return "Conversation conflict" }

func (e ErrTagConversationConflict) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}
