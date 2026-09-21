// Copyright (c) 2026 sh0jitmy <shjtmy@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
