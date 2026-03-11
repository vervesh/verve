package tome

import (
	"fmt"
	"math"
	"sort"

	"gonum.org/v1/gonum/mat"
)

// LSAIndex holds a Latent Semantic Analysis index for semantic search.
type LSAIndex struct {
	sessions    []Session        // all indexed sessions
	docVectors  [][]float64      // numDocs × k (document embeddings)
	termVectors [][]float64      // numTerms × k (V_k, for query projection)
	vocab       map[string]int   // term → column index
	idf         []float64        // IDF values per term
	numDocs     int
	dim         int
}

type lsaResult struct {
	Session Session
	Score   float64 // cosine similarity
}

// BuildLSA creates an LSA index from sessions using TF-IDF + SVD.
// dim is the target number of dimensions (capped by data size).
// Returns nil if there aren't enough sessions or terms.
func BuildLSA(sessions []Session, dim int) (*LSAIndex, error) {
	if len(sessions) < 2 {
		return nil, fmt.Errorf("need at least 2 sessions for LSA")
	}

	// Tokenize all sessions and compute document frequencies.
	docs := make([][]string, len(sessions))
	docFreq := map[string]int{}
	for i, s := range sessions {
		tokens := tokenize(sessionText(s))
		docs[i] = tokens
		seen := map[string]bool{}
		for _, t := range tokens {
			if !seen[t] {
				docFreq[t]++
				seen[t] = true
			}
		}
	}

	// Build vocabulary from terms appearing in ≥2 documents.
	vocab := map[string]int{}
	for term, freq := range docFreq {
		if freq >= 2 {
			vocab[term] = len(vocab)
		}
	}

	if len(vocab) == 0 {
		return nil, fmt.Errorf("no terms with document frequency >= 2")
	}

	numDocs := len(sessions)
	numTerms := len(vocab)

	// Compute IDF values.
	idf := make([]float64, numTerms)
	for term, idx := range vocab {
		idf[idx] = math.Log(float64(numDocs) / float64(docFreq[term]))
	}

	// Build TF-IDF matrix (documents × terms).
	data := make([]float64, numDocs*numTerms)
	A := mat.NewDense(numDocs, numTerms, data)
	for i, tokens := range docs {
		tf := map[string]int{}
		for _, t := range tokens {
			tf[t]++
		}
		for term, count := range tf {
			if j, ok := vocab[term]; ok {
				A.Set(i, j, float64(count)*idf[j])
			}
		}
	}

	// Determine dimensionality.
	k := dim
	if k > numDocs-1 {
		k = numDocs - 1
	}
	if k > numTerms-1 {
		k = numTerms - 1
	}
	if k < 1 {
		return nil, fmt.Errorf("insufficient data for dimensionality reduction")
	}

	// SVD decomposition.
	var svd mat.SVD
	ok := svd.Factorize(A, mat.SVDThin)
	if !ok {
		return nil, fmt.Errorf("SVD factorization failed")
	}

	var U, V mat.Dense
	svd.UTo(&U)
	svd.VTo(&V)
	values := svd.Values(nil)

	// Build document vectors: rows of U_k scaled by singular values.
	// This places documents in the concept space.
	docVectors := make([][]float64, numDocs)
	for i := range numDocs {
		vec := make([]float64, k)
		for j := range k {
			vec[j] = U.At(i, j) * values[j]
		}
		docVectors[i] = vec
	}

	// Store term projection matrix (V_k) for query mapping.
	termVectors := make([][]float64, numTerms)
	for i := range numTerms {
		vec := make([]float64, k)
		for j := range k {
			vec[j] = V.At(i, j)
		}
		termVectors[i] = vec
	}

	return &LSAIndex{
		sessions:    sessions,
		docVectors:  docVectors,
		termVectors: termVectors,
		vocab:       vocab,
		idf:         idf,
		numDocs:     numDocs,
		dim:         k,
	}, nil
}

// Query scores sessions against the given text using cosine similarity
// in the LSA concept space. Returns results sorted by score descending.
func (idx *LSAIndex) Query(text string, limit int) []lsaResult {
	tokens := tokenize(text)

	// Build query TF-IDF vector using the index vocabulary and IDF.
	queryVec := make([]float64, len(idx.vocab))
	tf := map[string]int{}
	for _, t := range tokens {
		tf[t]++
	}
	hasTerms := false
	for term, count := range tf {
		if j, ok := idx.vocab[term]; ok {
			queryVec[j] = float64(count) * idx.idf[j]
			hasTerms = true
		}
	}
	if !hasTerms {
		return nil
	}

	// Project query into concept space: q_proj = queryVec × V_k
	qProj := make([]float64, idx.dim)
	for j, tv := range idx.termVectors {
		qv := queryVec[j]
		if qv == 0 {
			continue
		}
		for d := range idx.dim {
			qProj[d] += qv * tv[d]
		}
	}

	qNorm := vecNorm(qProj)
	if qNorm == 0 {
		return nil
	}

	// Score each document by cosine similarity.
	type scored struct {
		index int
		score float64
	}
	results := make([]scored, 0, idx.numDocs)
	for i, dv := range idx.docVectors {
		dNorm := vecNorm(dv)
		if dNorm == 0 {
			continue
		}
		cosine := vecDot(qProj, dv) / (qNorm * dNorm)
		if cosine > 0 {
			results = append(results, scored{i, cosine})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	out := make([]lsaResult, len(results))
	for i, r := range results {
		out[i] = lsaResult{
			Session: idx.sessions[r.index],
			Score:   r.score,
		}
	}
	return out
}

func vecDot(a, b []float64) float64 {
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

func vecNorm(v []float64) float64 {
	return math.Sqrt(vecDot(v, v))
}
