package r2

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bid-hub/admin-backend/platform/env"
)

type PresignService struct {
	bucket        string
	prefix        string
	publicBaseURL string
	presigner     *awss3.PresignClient
	client        *awss3.Client
}

func NewPresignService(cfg *env.Config) (*PresignService, error) {
	if strings.TrimSpace(cfg.R2Bucket) == "" {
		return nil, fmt.Errorf("R2_BUCKET is required for presigned uploads")
	}
	if strings.TrimSpace(cfg.R2AccountID) == "" {
		return nil, fmt.Errorf("R2_ACCOUNT_ID is required for R2 endpoint")
	}
	if strings.TrimSpace(cfg.R2AccessKeyID) == "" || strings.TrimSpace(cfg.R2SecretAccessKey) == "" {
		return nil, fmt.Errorf("R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY are required for presigned uploads")
	}

	r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", strings.TrimSpace(cfg.R2AccountID))

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.R2AccessKeyID),
			strings.TrimSpace(cfg.R2SecretAccessKey),
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(r2Endpoint)
		o.UsePathStyle = false // R2 supports virtual-host style, avoiding signature mismatches on some SDK versions.
	})
	presigner := awss3.NewPresignClient(client)

	prefix := strings.Trim(strings.TrimSpace(cfg.R2Prefix), "/")
	if prefix == "" {
		prefix = "bid-media"
	}

	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.R2PublicDomain), "/")
	if publicBaseURL != "" && !strings.HasPrefix(publicBaseURL, "http") {
		publicBaseURL = "https://" + publicBaseURL
	}

	return &PresignService{
		bucket:        strings.TrimSpace(cfg.R2Bucket),
		prefix:        prefix,
		publicBaseURL: publicBaseURL,
		presigner:     presigner,
		client:        client,
	}, nil
}

type PresignedPut struct {
	Key       string
	SignedURL string
	PublicURL string
}

type PresignedGet struct {
	Key       string
	SignedURL string
}

func (s *PresignService) PresignPut(objectName string, contentType string, maxAge time.Duration) (*PresignedPut, error) {
	obj := strings.Trim(strings.TrimSpace(objectName), "/")
	if obj == "" {
		return nil, fmt.Errorf("objectName is required")
	}
	if strings.TrimSpace(contentType) == "" {
		return nil, fmt.Errorf("contentType is required")
	}
	if maxAge <= 0 {
		maxAge = 10 * time.Minute
	}

	key := s.prefix + "/" + obj

	input := &awss3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	out, err := s.presigner.PresignPutObject(context.Background(), input, func(po *awss3.PresignOptions) {
		po.Expires = maxAge
	})
	if err != nil {
		return nil, err
	}

	publicURL := ""
	if s.publicBaseURL != "" {
		publicURL = s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
	}

	return &PresignedPut{
		Key:       key,
		SignedURL: out.URL,
		PublicURL: publicURL,
	}, nil
}

// Upload uploads a file directly to R2 server-side (no presigned URL needed).
// Use this to avoid browser CORS/TLS issues when uploading from backend.
func (s *PresignService) Upload(objectName, contentType string, data []byte) (string, error) {
	obj := strings.Trim(strings.TrimSpace(objectName), "/")
	if obj == "" {
		return "", fmt.Errorf("objectName is required")
	}

	key := s.prefix + "/" + obj

	_, err := s.client.PutObject(context.Background(), &awss3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return "", err
	}

	publicURL := ""
	if s.publicBaseURL != "" {
		publicURL = s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
	}
	return publicURL, nil
}

func (s *PresignService) PresignGet(key string, maxAge time.Duration) (*PresignedGet, error) {
	k := strings.Trim(strings.TrimSpace(key), "/")
	if k == "" {
		return nil, fmt.Errorf("key is required")
	}

	// For R2 with a custom domain, serve files publicly  no signing needed.
	publicURL := ""
	if s.publicBaseURL != "" {
		publicURL = s.publicBaseURL + "/" + k
	}

	return &PresignedGet{Key: k, SignedURL: publicURL}, nil
}
