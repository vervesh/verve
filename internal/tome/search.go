package tome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Search finds sessions matching the query. Uses hybrid BM25+LSA scoring
// when an LSA index is available (≥2 sessions), falling back to BM25-only.
func (t *Tome) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	if opts.BM25Only {
		return t.searchBM25(ctx, query, opts)
	}

	idx := t.ensureLSA(ctx)
	if idx == nil {
		return t.searchBM25(ctx, query, opts)
	}

	return t.searchHybrid(ctx, query, opts, idx)
}

// searchBM25 performs a keyword search using FTS5 BM25 ranking.
func (t *Tome) searchBM25(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}

	q := `
		SELECT s.id, s.summary, s.learnings, s.content, s.tags, s.files, s.branch, s.status, s.transcript_hash, s.user, s.created_at,
		       session_fts.rank
		FROM session_fts
		JOIN session s ON session_fts.rowid = s.rowid
		WHERE session_fts MATCH ?`
	args := []any{query}

	if opts.Status != "" {
		q += ` AND s.status = ?`
		args = append(args, opts.Status)
	}
	if opts.FilePattern != "" {
		q += ` AND s.files LIKE ?`
		args = append(args, "%"+opts.FilePattern+"%")
	}

	q += ` ORDER BY session_fts.rank LIMIT ?`
	args = append(args, limit)

	rows, err := t.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var tagsJSON, filesJSON string
		var createdAt int64
		var transcriptHash sql.NullString

		err := rows.Scan(
			&r.Session.ID, &r.Session.Summary, &r.Session.Learnings, &r.Session.Content,
			&tagsJSON, &filesJSON, &r.Session.Branch, &r.Session.Status, &transcriptHash, &r.Session.User, &createdAt,
			&r.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		if transcriptHash.Valid {
			r.Session.TranscriptHash = transcriptHash.String
		}

		_ = json.Unmarshal([]byte(tagsJSON), &r.Session.Tags)
		_ = json.Unmarshal([]byte(filesJSON), &r.Session.Files)
		if r.Session.Tags == nil {
			r.Session.Tags = []string{}
		}
		if r.Session.Files == nil {
			r.Session.Files = []string{}
		}
		r.Session.CreatedAt = time.Unix(createdAt, 0)
		results = append(results, r)
	}

	return results, rows.Err()
}

// searchHybrid combines BM25 keyword scores with LSA semantic scores.
// final_score = 0.4 × normalize(bm25) + 0.6 × normalize(lsa)
func (t *Tome) searchHybrid(ctx context.Context, query string, opts SearchOpts, idx *LSAIndex) ([]SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}

	// Get BM25 results with generous pool.
	bm25Opts := SearchOpts{
		Status:      opts.Status,
		FilePattern: opts.FilePattern,
		Limit:       50,
		BM25Only:    true,
	}
	bm25Results, err := t.searchBM25(ctx, query, bm25Opts)
	if err != nil {
		// FTS5 query failed (e.g. syntax error) — try LSA only.
		bm25Results = nil
	}

	// Get LSA scores for all indexed sessions.
	lsaResults := idx.Query(query, 0)

	// Merge into a combined candidate set.
	type candidate struct {
		session Session
		bm25    float64 // higher = better (negated FTS5 rank)
		lsa     float64 // higher = better (cosine similarity)
	}
	byID := map[string]*candidate{}

	for _, r := range bm25Results {
		byID[r.Session.ID] = &candidate{
			session: r.Session,
			bm25:    -r.Score, // FTS5 rank is negative; negate so higher = better
		}
	}

	for _, r := range lsaResults {
		if c, ok := byID[r.Session.ID]; ok {
			c.lsa = r.Score
		} else {
			// LSA-only match — apply filters.
			if opts.Status != "" && r.Session.Status != opts.Status {
				continue
			}
			if opts.FilePattern != "" && !matchesFilePattern(r.Session.Files, opts.FilePattern) {
				continue
			}
			byID[r.Session.ID] = &candidate{
				session: r.Session,
				lsa:     r.Score,
			}
		}
	}

	if len(byID) == 0 {
		return nil, nil
	}

	// Find max scores for normalization.
	var maxBM25, maxLSA float64
	for _, c := range byID {
		if c.bm25 > maxBM25 {
			maxBM25 = c.bm25
		}
		if c.lsa > maxLSA {
			maxLSA = c.lsa
		}
	}

	// Compute combined scores and sort.
	type scored struct {
		session Session
		score   float64
	}
	candidates := make([]scored, 0, len(byID))
	for _, c := range byID {
		var bm25Norm, lsaNorm float64
		if maxBM25 > 0 {
			bm25Norm = c.bm25 / maxBM25
		}
		if maxLSA > 0 {
			lsaNorm = c.lsa / maxLSA
		}
		combined := 0.4*bm25Norm + 0.6*lsaNorm
		candidates = append(candidates, scored{c.session, combined})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = SearchResult{Session: c.session, Score: c.score}
	}
	return results, nil
}

func matchesFilePattern(files []string, pattern string) bool {
	for _, f := range files {
		if strings.Contains(f, pattern) {
			return true
		}
	}
	return false
}
