package builder_test

import (
	"testing"

	"github.com/sh0jitmy/dewyctl/internal/builder"
)

func TestStandardPlatforms(t *testing.T) {
	if len(builder.StandardPlatforms) == 0 {
		t.Fatal("expected StandardPlatforms to contain platforms, got 0")
	}

	expectedCount := 4
	if len(builder.StandardPlatforms) != expectedCount {
		t.Errorf("expected %d standard platforms, got %d", expectedCount, len(builder.StandardPlatforms))
	}

	platforms := make(map[string]bool)
	for _, p := range builder.StandardPlatforms {
		key := p.OS + "/" + p.Arch
		platforms[key] = true
	}

	expected := []string{
		"linux/amd64",
		"linux/arm64",
		"darwin/amd64",
		"darwin/arm64",
	}

	for _, ep := range expected {
		if !platforms[ep] {
			t.Errorf("expected platform %s not found in StandardPlatforms", ep)
		}
	}
}
