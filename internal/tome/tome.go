package tome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/joshjon/kit/sqlitedb"

	"github.com/joshjon/verve/internal/tome/migrations"
)

const defaultLSADim = 128

// Tome provides session memory backed by a local SQLite database with FTS5 search.
type Tome struct {
	db  *sql.DB
	dir string

	mu  sync.Mutex
	lsa *LSAIndex // cached, rebuilt lazily when stale
}

// Open opens (or creates) a Tome database in the given directory.
func Open(dir string) (*Tome, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sqlitedb.Open(ctx, sqlitedb.WithDir(dir), sqlitedb.WithDBName("data"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := sqlitedb.Migrate(db, migrations.FS); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Tome{db: db, dir: dir}, nil
}

// Dir returns the data directory path.
func (t *Tome) Dir() string {
	return t.dir
}

// Close closes the database connection.
func (t *Tome) Close() error {
	return t.db.Close()
}

// Log returns the most recent sessions ordered by creation time (newest first).
func (t *Tome) Log(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := t.db.QueryContext(ctx, `
		SELECT id, summary, learnings, content, tags, files, branch, status, transcript_hash, user, created_at
		FROM session
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// Get returns a single session by ID.
func (t *Tome) Get(ctx context.Context, id string) (Session, error) {
	row := t.db.QueryRowContext(ctx, `
		SELECT id, summary, learnings, content, tags, files, branch, status, transcript_hash, user, created_at
		FROM session
		WHERE id = ?
	`, id)
	return scanSession(row)
}

// BuildIndex forces a rebuild of the LSA index and returns stats.
func (t *Tome) BuildIndex(ctx context.Context) (numDocs, numTerms, dim int, err error) {
	sessions, err := t.allSessions(ctx)
	if err != nil {
		return 0, 0, 0, err
	}

	idx, err := BuildLSA(sessions, defaultLSADim)
	if err != nil {
		return len(sessions), 0, 0, err
	}

	t.mu.Lock()
	t.lsa = idx
	t.mu.Unlock()

	return idx.numDocs, len(idx.vocab), idx.dim, nil
}

// ensureLSA returns a fresh LSA index, rebuilding if stale or missing.
// Returns nil (no error) when there's insufficient data for LSA.
func (t *Tome) ensureLSA(ctx context.Context) *LSAIndex {
	t.mu.Lock()
	defer t.mu.Unlock()

	var count int
	_ = t.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM session").Scan(&count)

	if t.lsa != nil && t.lsa.numDocs == count {
		return t.lsa
	}

	if count < 2 {
		return nil
	}

	sessions, err := t.allSessions(ctx)
	if err != nil {
		return nil
	}

	idx, err := BuildLSA(sessions, defaultLSADim)
	if err != nil {
		return nil
	}

	t.lsa = idx
	return idx
}

// allSessions loads every session from the database.
func (t *Tome) allSessions(ctx context.Context) ([]Session, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT id, summary, learnings, content, tags, files, branch, status, transcript_hash, user, created_at
		FROM session
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(row scanner) (Session, error) {
	var s Session
	var tagsJSON, filesJSON string
	var createdAt int64
	var transcriptHash sql.NullString

	err := row.Scan(&s.ID, &s.Summary, &s.Learnings, &s.Content, &tagsJSON, &filesJSON, &s.Branch, &s.Status, &transcriptHash, &s.User, &createdAt)
	if err != nil {
		return Session{}, err
	}

	if transcriptHash.Valid {
		s.TranscriptHash = transcriptHash.String
	}

	_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
	_ = json.Unmarshal([]byte(filesJSON), &s.Files)
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Files == nil {
		s.Files = []string{}
	}
	s.CreatedAt = time.Unix(createdAt, 0)
	return s, nil
}
