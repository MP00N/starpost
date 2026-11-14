package vertex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetModelRegionReturnsConfiguredRegion(t *testing.T) {
	other := `{"gpt-4":"us-central1","default":"global"}`

	assert.Equal(t, "us-central1", GetModelRegion(other, "gpt-4"))
	assert.Equal(t, "global", GetModelRegion(other, "unknown-model"))
}

func TestGetModelRegionFallsBackForNonStringValues(t *testing.T) {
	// A non-string region value used to panic the unchecked type assertion.
	other := `{"gpt-4":123,"default":"global"}`

	assert.Equal(t, "global", GetModelRegion(other, "gpt-4"))
}

func TestGetModelRegionPassesThroughPlainString(t *testing.T) {
	assert.Equal(t, "us-east1", GetModelRegion("us-east1", "gpt-4"))
}
