package tome

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeRepo(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"https url", "https://github.com/joshjon/verve.git", "joshjon/verve"},
		{"https url no .git", "https://github.com/joshjon/verve", "joshjon/verve"},
		{"http url", "http://github.com/joshjon/verve.git", "joshjon/verve"},
		{"ssh url", "git@github.com:joshjon/verve.git", "joshjon/verve"},
		{"ssh url no .git", "git@github.com:joshjon/verve", "joshjon/verve"},
		{"plain owner/name", "joshjon/verve", "joshjon/verve"},
		{"uppercase", "JoshJon/Verve", "joshjon/verve"},
		{"uppercase https", "https://github.com/JoshJon/Verve.git", "joshjon/verve"},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"no slash", "justarepo", ""},
		{"too many slashes", "a/b/c", ""},
		{"missing owner", "/name", ""},
		{"missing name", "owner/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeRepo(tt.raw))
		})
	}
}
