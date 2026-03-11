package tome

import (
	"regexp"
	"strings"
)

var wordPattern = regexp.MustCompile(`[a-z0-9]+`)

// tokenize processes text into normalized tokens with stop words removed.
func tokenize(text string) []string {
	words := wordPattern.FindAllString(strings.ToLower(text), -1)
	tokens := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) <= 1 {
			continue
		}
		if stopWords[w] {
			continue
		}
		tokens = append(tokens, w)
	}
	return tokens
}

// sessionText returns the searchable text content of a session.
func sessionText(s Session) string {
	parts := []string{s.Summary, s.Learnings}
	if len(s.Tags) > 0 {
		parts = append(parts, strings.Join(s.Tags, " "))
	}
	return strings.Join(parts, " ")
}

var stopWords = map[string]bool{
	"the": true, "be": true, "to": true, "of": true, "and": true,
	"in": true, "that": true, "have": true, "it": true, "for": true,
	"not": true, "on": true, "with": true, "he": true, "as": true,
	"you": true, "do": true, "at": true, "this": true, "but": true,
	"his": true, "by": true, "from": true, "they": true, "we": true,
	"say": true, "her": true, "she": true, "or": true, "an": true,
	"will": true, "my": true, "one": true, "all": true, "would": true,
	"there": true, "their": true, "what": true, "so": true, "up": true,
	"out": true, "if": true, "about": true, "who": true, "get": true,
	"which": true, "go": true, "me": true, "when": true, "make": true,
	"can": true, "like": true, "no": true, "just": true, "him": true,
	"know": true, "take": true, "people": true, "into": true, "your": true,
	"some": true, "could": true, "them": true, "see": true, "other": true,
	"than": true, "then": true, "now": true, "its": true, "also": true,
	"after": true, "use": true, "how": true, "our": true, "well": true,
	"way": true, "even": true, "new": true, "want": true, "because": true,
	"any": true, "these": true, "give": true, "most": true, "us": true,
	"is": true, "are": true, "was": true, "were": true, "been": true,
	"being": true, "am": true, "has": true, "had": true, "did": true,
	"does": true, "done": true, "should": true, "may": true, "might": true,
	"shall": true, "must": true, "need": true, "ought": true, "here": true,
	"where": true, "why": true, "very": true, "too": true, "only": true,
	"own": true, "same": true, "both": true, "each": true, "few": true,
	"more": true, "such": true, "over": true, "again": true, "under": true,
	"further": true, "before": true, "between": true, "through": true,
	"during": true, "above": true, "below": true, "down": true,
	"while": true, "until": true, "against": true, "those": true,
	"every": true, "much": true, "many": true, "still": true,
}
