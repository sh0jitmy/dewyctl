package s3_test

import (
	"context"
	"testing"

	"github.com/sh0jitmy/dewyctl/internal/s3"
)

func TestUploadOptionsValidation(t *testing.T) {
	ctx := context.Background()

	// Missing bucket
	err := s3.UploadArtifact(ctx, s3.UploadOptions{})
	if err == nil {
		t.Error("expected error for missing bucket, got nil")
	}

	// Missing credentials
	err = s3.UploadArtifact(ctx, s3.UploadOptions{Bucket: "my-bucket"})
	if err == nil {
		t.Error("expected error for missing credentials, got nil")
	}

	// CheckBucketAccess missing bucket
	err = s3.CheckBucketAccess(ctx, s3.UploadOptions{})
	if err == nil {
		t.Error("expected error for CheckBucketAccess with empty bucket, got nil")
	}
}
