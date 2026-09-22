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

package template_test

import (
	"strings"
	"testing"

	"github.com/sh0jitmy/dewyctl/internal/template"
)

func TestTemplatesNotEmpty(t *testing.T) {
	cfg := template.Config{
		AppName:    "dewy-app",
		Port:       "8080",
		S3Bucket:   "test-bucket",
		S3Endpoint: "https://s3.example.com",
		S3Region:   "jp-east-1",
		BinaryDir:  "/opt/dewy-app",
		GitOwner:   "test-owner",
		GitRepo:    "test-repo",
	}

	sysd := template.SystemdServiceTemplate(cfg)
	if !strings.Contains(sysd, "test-bucket") {
		t.Errorf("SystemdServiceTemplate does not contain expected bucket string")
	}

	tagpr := template.TagprWorkflowTemplate(cfg)
	if !strings.Contains(tagpr, "https://s3.example.com") {
		t.Errorf("TagprWorkflowTemplate does not contain expected endpoint")
	}
	if !strings.Contains(tagpr, "jp-east-1") {
		t.Errorf("TagprWorkflowTemplate does not contain expected region")
	}

	goreleaser := template.GoReleaserTemplate(cfg)
	if !strings.Contains(goreleaser, "dewy-app") {
		t.Errorf("GoReleaserTemplate does not contain expected app name")
	}
}
