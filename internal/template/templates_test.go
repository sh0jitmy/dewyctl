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
