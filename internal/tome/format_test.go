package tome

import "testing"

func TestMatchSnippet(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog. " +
		"Authentication was broken because the token validation middleware " +
		"was not checking expiry dates correctly. Fixed by adding a time check."

	tests := []struct {
		name    string
		query   string
		window  int
		wantHit bool
	}{
		{"single term match", "authentication", 50, true},
		{"multi term earliest wins", "expiry token", 60, true},
		{"no match", "kubernetes", 50, false},
		{"empty content", "", 50, false},
		{"empty query", "auth", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := content
			if tt.name == "empty content" {
				c = ""
			}
			result := matchSnippet(c, tt.query, tt.window)
			if tt.wantHit && result == "" {
				t.Error("expected snippet, got empty string")
			}
			if !tt.wantHit && result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
		})
	}
}

func TestMatchSnippetCaseInsensitive(t *testing.T) {
	content := "The ERROR occurred in the Authentication layer"
	snippet := matchSnippet(content, "error", 100)
	if snippet == "" {
		t.Error("expected case-insensitive match")
	}
}

func TestMatchSnippetCollapsesWhitespace(t *testing.T) {
	content := "line one\n\n\nthe  match   here\n\nline two"
	snippet := matchSnippet(content, "match", 100)
	if snippet == "" {
		t.Fatal("expected snippet")
	}
	// Should not contain consecutive whitespace.
	for i := 0; i < len(snippet)-1; i++ {
		if snippet[i] == ' ' && snippet[i+1] == ' ' {
			t.Error("snippet contains consecutive spaces")
			break
		}
	}
}
