package epic

import (
	"strings"
	"testing"

	"github.com/cohesivestack/valgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEpicID(t *testing.T) {
	id := NewEpicID()
	s := id.String()
	assert.True(t, strings.HasPrefix(s, "epc_"), "expected epc_ prefix, got %s", s)
}

func TestParseEpicID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid ID", NewEpicID().String(), false},
		{"empty string", "", true},
		{"wrong prefix", "tsk_01HQXYZ0000000000000000000", true},
		{"garbage", "not-an-id", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseEpicID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.input, parsed.String())
			}
		})
	}
}

func TestMustParseEpicID_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustParseEpicID("invalid")
	})
}

func TestEpicIDValidator(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid ID", NewEpicID().String(), false},
		{"empty string", "", true},
		{"invalid format", "bad", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := valgo.Is(EpicIDValidator(tt.input, "id")).ToError()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
