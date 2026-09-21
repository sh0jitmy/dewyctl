package s3

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3service "github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadOptions represents S3 upload parameters
type UploadOptions struct {
	Endpoint        string
	Region          string
	Bucket          string
	Key             string
	AccessKeyID     string
	SecretAccessKey string
	Data            []byte
}

// UploadArtifact uploads an artifact to S3-compatible object storage
func UploadArtifact(ctx context.Context, opts UploadOptions) error {
	if opts.AccessKeyID == "" {
		opts.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if opts.SecretAccessKey == "" {
		opts.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if opts.Region == "" {
		opts.Region = os.Getenv("S3_REGION")
		if opts.Region == "" {
			opts.Region = "jp-east-1"
		}
	}
	if opts.Endpoint == "" {
		opts.Endpoint = os.Getenv("S3_ENDPOINT")
	}
	if opts.Bucket == "" {
		opts.Bucket = os.Getenv("S3_BUCKET")
	}

	if opts.Bucket == "" {
		return fmt.Errorf("S3 bucket name is required (via flag or S3_BUCKET)")
	}
	if opts.AccessKeyID == "" || opts.SecretAccessKey == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	// Custom resolver for S3 endpoint (e.g. Sakura Object Storage or MinIO)
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if opts.Endpoint != "" {
			return aws.Endpoint{
				URL:               opts.Endpoint,
				SigningRegion:     opts.Region,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(opts.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3service.NewFromConfig(cfg, func(o *s3service.Options) {
		// Sakura Object Storage and MinIO typically work best with path-style
		o.UsePathStyle = true
	})

	fmt.Printf("--> Uploading %s to s3://%s/%s (endpoint: %s)...\n", opts.Key, opts.Bucket, opts.Key, opts.Endpoint)
	_, err = client.PutObject(ctx, &s3service.PutObjectInput{
		Bucket: aws.String(opts.Bucket),
		Key:    aws.String(opts.Key),
		Body:   bytes.NewReader(opts.Data),
	})
	if err != nil {
		return fmt.Errorf("failed to put object to S3: %w", err)
	}

	fmt.Printf("--> Successfully uploaded to s3://%s/%s\n", opts.Bucket, opts.Key)
	return nil
}

// CheckBucketAccess tests read/write access to the specified S3 bucket and removes test object
func CheckBucketAccess(ctx context.Context, opts UploadOptions) error {
	if opts.Bucket == "" {
		return fmt.Errorf("S3 bucket name is required")
	}
	if opts.AccessKeyID == "" || opts.SecretAccessKey == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}
	if opts.Region == "" {
		opts.Region = "jp-east-1"
	}
	if opts.Endpoint == "" {
		opts.Endpoint = "https://s3.tky01.sakurastorage.jp"
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               opts.Endpoint,
			SigningRegion:     opts.Region,
			HostnameImmutable: true,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(opts.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3service.NewFromConfig(cfg, func(o *s3service.Options) {
		o.UsePathStyle = true
	})

	testKey := ".dewy-ping"
	// 1. Put test object
	_, err = client.PutObject(ctx, &s3service.PutObjectInput{
		Bucket: aws.String(opts.Bucket),
		Key:    aws.String(testKey),
		Body:   bytes.NewReader([]byte("dewy-ping-ok")),
	})
	if err != nil {
		return fmt.Errorf("PutObject check failed: %w", err)
	}

	// 2. Clean up test object
	_, _ = client.DeleteObject(ctx, &s3service.DeleteObjectInput{
		Bucket: aws.String(opts.Bucket),
		Key:    aws.String(testKey),
	})

	return nil
}
