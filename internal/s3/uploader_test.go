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
