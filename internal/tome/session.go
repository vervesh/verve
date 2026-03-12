package tome

import "time"

// Session represents a recorded agent session.
type Session struct {
	ID             string    `json:"id"`
	Summary        string    `json:"summary"`
	Learnings      string    `json:"learnings"`
	Content        string    `json:"content"`
	Tags           []string  `json:"tags"`
	Files          []string  `json:"files"`
	Branch         string    `json:"branch"`
	Status         string    `json:"status"`
	TranscriptHash string    `json:"transcript_hash,omitempty"`
	User           string    `json:"user"`
	CreatedAt      time.Time `json:"created_at"`
}

// SearchOpts configures a search query.
type SearchOpts struct {
	FilePattern string // filter sessions by files touched
	Status      string // filter by outcome
	Limit       int    // max results (default 5)
	BM25Only    bool   // force BM25-only mode (skip LSA)
}

// SearchResult is a session matched by a search query.
type SearchResult struct {
	Session Session `json:"session"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet,omitempty"` // context window around query match
}
