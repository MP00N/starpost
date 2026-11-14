package common

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalLogPreviewKeepsShortContent(t *testing.T) {
	short := "hello 世界"
	assert.Equal(t, short, LocalLogPreview(short))
}

func TestLocalLogPreviewTruncatesOnRuneBoundary(t *testing.T) {
	if DebugEnabled {
		t.Skip("debug logging disables truncation")
	}

	// Each rune is three bytes, so the byte limit lands in the middle of a rune.
	content := strings.Repeat("中", LocalLogContentLimit)
	got := LocalLogPreview(content)

	require.True(t, utf8.ValidString(got), "truncated preview must remain valid UTF-8")
	assert.Contains(t, got, "truncated")
	// The cut lands on the rune boundary at byte 2046 (682 runes), not 2048.
	assert.True(t, strings.HasPrefix(got, strings.Repeat("中", 682)))
}
