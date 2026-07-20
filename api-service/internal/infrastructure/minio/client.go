package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	MaxUploadSize   int64
}

type Client struct {
	client     *minio.Client
	bucketName string
	maxSize    int64
	logger     *zap.Logger
}

func NewClient(cfg Config, logger *zap.Logger) (*Client, error) {
	// Credential mode selection:
	//   - static:       both access key and secret key are set (compose/kind/MinIO).
	//   - ambient/IRSA:  keys are absent -> fall back to the AWS credential chain.
	// credentials.NewIAM("") resolves EKS IRSA via the web-identity path: it reads
	// AWS_WEB_IDENTITY_TOKEN_FILE + AWS_ROLE_ARN (injected by EKS) and exchanges the
	// projected service-account token with STS. It also covers the EC2/ECS metadata
	// providers, so it is the correct single "ambient" provider here.
	// NOTE: presigned URLs generated with temporary (IRSA) creds embed the STS session
	// token as a query param; the round-trip under IRSA still needs live verification
	// (planned as part of EXP-50's smoke leg — not yet run).
	var creds *credentials.Credentials
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		creds = credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, "")
		logger.Info("MinIO credential mode: static")
	} else {
		creds = credentials.NewIAM("")
		logger.Info("MinIO credential mode: ambient/IRSA")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		if err := client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Info("Created MinIO bucket", zap.String("bucket", cfg.BucketName))
	}

	return &Client{
		client:     client,
		bucketName: cfg.BucketName,
		maxSize:    cfg.MaxUploadSize,
		logger:     logger.Named("minio"),
	}, nil
}

func (c *Client) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	if size > c.maxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", size, c.maxSize)
	}

	_, err := c.client.PutObject(ctx, c.bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	c.logger.Debug("Uploaded object",
		zap.String("bucket", c.bucketName),
		zap.String("object", objectName),
		zap.Int64("size", size),
	)

	return nil
}

func (c *Client) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	obj, err := c.client.GetObject(ctx, c.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return obj, nil
}

func (c *Client) Delete(ctx context.Context, objectName string) error {
	if err := c.client.RemoveObject(ctx, c.bucketName, objectName, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	c.logger.Debug("Deleted object",
		zap.String("bucket", c.bucketName),
		zap.String("object", objectName),
	)

	return nil
}

func (c *Client) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	url, err := c.client.PresignedGetObject(ctx, c.bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.client.BucketExists(ctx, c.bucketName)
	return err
}

func (c *Client) BucketName() string {
	return c.bucketName
}
