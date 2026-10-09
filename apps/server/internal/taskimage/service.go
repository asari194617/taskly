package taskimage

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const DefaultMaxBytes int64 = 10 * 1024 * 1024

var allowedContentTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/gif":  {},
}

type Service struct {
	bucket    string
	maxBytes  int64
	client    *s3.Client
	presigner *s3.PresignClient
}

type Upload struct {
	Key         string            `json:"key"`
	URL         string            `json:"upload_url"`
	Headers     map[string]string `json:"headers"`
	ExpiresIn   int64             `json:"expires_in"`
	MaxBytes    int64             `json:"max_bytes"`
	ContentType string            `json:"content_type"`
}

func NewFromEnv() (*Service, error) {
	endpoint := strings.TrimRight(utils.Env("R2_ENDPOINT", ""), "/")
	bucket := strings.TrimSpace(utils.Env("R2_BUCKET", ""))
	accessKey := strings.TrimSpace(utils.Env("R2_ACCESS_KEY_ID", ""))
	secretKey := strings.TrimSpace(utils.Env("R2_SECRET_ACCESS_KEY", ""))
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("task image storage is not configured")
	}
	maxBytes := DefaultMaxBytes
	if raw := strings.TrimSpace(utils.Env("TASK_IMAGE_MAX_BYTES", "")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("TASK_IMAGE_MAX_BYTES must be a positive integer")
		}
		maxBytes = parsed
	}
	cfg := aws.Config{
		Region:                     "auto",
		BaseEndpoint:               aws.String(endpoint),
		Credentials:                credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	}
	client := s3.NewFromConfig(cfg)
	return &Service{
		bucket:    bucket,
		maxBytes:  maxBytes,
		client:    client,
		presigner: s3.NewPresignClient(client),
	}, nil
}

func (s *Service) PresignUpload(ctx context.Context, userID uint, contentType string, size int64) (Upload, error) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !AllowedContentType(contentType) {
		return Upload{}, fmt.Errorf("unsupported image type")
	}
	if size <= 0 || size > s.maxBytes {
		return Upload{}, fmt.Errorf("image must be between 1 and %d bytes", s.maxBytes)
	}
	key := fmt.Sprintf("task-images/%d/%s", userID, uuid.NewString())
	expires := 10 * time.Minute
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		Metadata: map[string]string{
			"owner-id": strconv.FormatUint(uint64(userID), 10),
		},
	}, func(options *s3.PresignOptions) {
		options.Expires = expires
	})
	if err != nil {
		return Upload{}, fmt.Errorf("presign upload: %w", err)
	}
	headers := make(map[string]string, len(result.SignedHeader))
	for name, values := range result.SignedHeader {
		if strings.EqualFold(name, "host") || strings.EqualFold(name, "content-length") {
			continue
		}
		headers[name] = strings.Join(values, ",")
	}
	return Upload{
		Key:         key,
		URL:         result.URL,
		Headers:     headers,
		ExpiresIn:   int64(expires.Seconds()),
		MaxBytes:    s.maxBytes,
		ContentType: contentType,
	}, nil
}

func (s *Service) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return fmt.Errorf("invalid image key")
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	return nil
}

func (s *Service) ValidateUploaded(ctx context.Context, userID uint, key string) error {
	if !OwnedKey(userID, key) {
		return fmt.Errorf("invalid image key")
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("image upload not found: %w", err)
	}
	if head.ContentLength == nil || *head.ContentLength <= 0 || *head.ContentLength > s.maxBytes {
		return fmt.Errorf("uploaded image exceeds the size limit")
	}
	if head.ContentType == nil || !AllowedContentType(*head.ContentType) {
		return fmt.Errorf("uploaded image has an unsupported type")
	}
	if head.Metadata["owner-id"] != strconv.FormatUint(uint64(userID), 10) {
		return fmt.Errorf("uploaded image owner does not match")
	}
	return nil
}

func (s *Service) PresignView(ctx context.Context, key string) (string, error) {
	if !ValidKey(key) {
		return "", fmt.Errorf("invalid image key")
	}
	expires := 15 * time.Minute
	result, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(options *s3.PresignOptions) {
		options.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("presign view: %w", err)
	}
	return result.URL, nil
}

func AllowedContentType(contentType string) bool {
	_, ok := allowedContentTypes[strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))]
	return ok
}

func OwnedKey(userID uint, key string) bool {
	return strings.HasPrefix(key, fmt.Sprintf("task-images/%d/", userID)) && ValidKey(key)
}

func ValidKey(key string) bool {
	clean := path.Clean(key)
	parts := strings.Split(clean, "/")
	if clean != key || len(parts) != 3 || parts[0] != "task-images" {
		return false
	}
	if _, err := strconv.ParseUint(parts[1], 10, 64); err != nil {
		return false
	}
	_, err := uuid.Parse(parts[2])
	return err == nil
}
