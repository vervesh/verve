package tome

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// jsonlSession is the wire format for sessions in JSONL files on git branches.
type jsonlSession struct {
	ID             string   `json:"id"`
	Summary        string   `json:"summary"`
	Learnings      string   `json:"learnings,omitempty"`
	Content        string   `json:"content,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Files          []string `json:"files,omitempty"`
	Branch         string   `json:"branch,omitempty"`
	Status         string   `json:"status"`
	TranscriptHash string   `json:"transcript_hash,omitempty"`
	User           string   `json:"user,omitempty"`
	Repo           string   `json:"repo,omitempty"`
	CreatedAt      string   `json:"created_at"`
}

func sessionToJSONL(s Session) jsonlSession {
	return jsonlSession{
		ID:             s.ID,
		Summary:        s.Summary,
		Learnings:      s.Learnings,
		Content:        s.Content,
		Tags:           s.Tags,
		Files:          s.Files,
		Branch:         s.Branch,
		Status:         s.Status,
		TranscriptHash: s.TranscriptHash,
		User:           s.User,
		Repo:           s.Repo,
		CreatedAt:      s.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func jsonlToSession(j jsonlSession) (Session, error) {
	t, err := time.Parse(time.RFC3339, j.CreatedAt)
	if err != nil {
		return Session{}, fmt.Errorf("parse created_at %q: %w", j.CreatedAt, err)
	}
	s := Session{
		ID:             j.ID,
		Summary:        j.Summary,
		Learnings:      j.Learnings,
		Content:        j.Content,
		Tags:           j.Tags,
		Files:          j.Files,
		Branch:         j.Branch,
		Status:         j.Status,
		TranscriptHash: j.TranscriptHash,
		User:           j.User,
		Repo:           j.Repo,
		CreatedAt:      t,
	}
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Files == nil {
		s.Files = []string{}
	}
	return s, nil
}

// encodeJSONL writes sessions as newline-delimited JSON.
func encodeJSONL(w io.Writer, sessions []Session) error {
	enc := json.NewEncoder(w)
	for _, s := range sessions {
		if err := enc.Encode(sessionToJSONL(s)); err != nil {
			return fmt.Errorf("encode session %s: %w", s.ID, err)
		}
	}
	return nil
}

// decodeJSONL reads sessions from newline-delimited JSON.
func decodeJSONL(r io.Reader) ([]Session, error) {
	var sessions []Session
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var j jsonlSession
		if err := json.Unmarshal(line, &j); err != nil {
			return nil, fmt.Errorf("decode jsonl line: %w", err)
		}
		s, err := jsonlToSession(j)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, scanner.Err()
}
