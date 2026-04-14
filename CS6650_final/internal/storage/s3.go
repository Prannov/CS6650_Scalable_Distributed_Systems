package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(bucket, region string) (*S3Store, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &S3Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

// Upload puts bytes to S3 and returns a presigned URL.
func (s *S3Store) Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}
	return s.Presign(ctx, key)
}

// Presign generates a presigned GET URL for an existing key.
func (s *S3Store) Presign(ctx context.Context, key string) (string, error) {
	presigner := s3.NewPresignClient(s.client)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(7*24*time.Hour))
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return req.URL, nil
}

// UploadMultipart streams body to S3 using multipart upload.
// Returns presigned URL. Safe to call with large bodies.
func (s *S3Store) UploadMultipart(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	// Initiate multipart upload
	createOut, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("create multipart: %w", err)
	}
	uploadID := *createOut.UploadId

	// Upload parts in 10MB chunks
	const partSize = 10 * 1024 * 1024
	var parts []types.CompletedPart
	partNum := int32(1)
	buf := make([]byte, partSize)

	for {
		n, readErr := io.ReadFull(body, buf)
		if n == 0 {
			break
		}
		chunk := buf[:n]

		uploadOut, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(s.bucket),
			Key:        aws.String(key),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(partNum),
			Body:       &byteReader{data: chunk, pos: 0},
		})
		if err != nil {
			s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(s.bucket),
				Key:      aws.String(key),
				UploadId: aws.String(uploadID),
			})
			return "", fmt.Errorf("upload part %d: %w", partNum, err)
		}
		parts = append(parts, types.CompletedPart{
			ETag:       uploadOut.ETag,
			PartNumber: aws.Int32(partNum),
		})
		partNum++

		if readErr == io.ErrUnexpectedEOF || readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read: %w", readErr)
		}
	}

	// Complete multipart upload
	_, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		return "", fmt.Errorf("complete multipart: %w", err)
	}

	return s.Presign(ctx, key)
}

// Delete removes an object from S3.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// byteReader wraps a byte slice as io.ReadSeeker for UploadPart
type byteReader struct {
	data []byte
	pos  int
}

func (b *byteReader) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}
