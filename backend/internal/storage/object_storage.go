package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

// ObjectStorage 封装 MinIO 操作
type ObjectStorage struct {
	client *minio.Client
	bucket string
}

// NewObjectStorage 创建 ObjectStorage 实例
func NewObjectStorage(client *minio.Client, bucket string) *ObjectStorage {
	return &ObjectStorage{client: client, bucket: bucket}
}

// PutObject 上传文件到 MinIO
func (s *ObjectStorage) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}
	return nil
}

// GetObject 从 MinIO 下载文件
func (s *ObjectStorage) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("下载文件失败: %w", err)
	}
	return obj, nil
}

// RemoveObject 从 MinIO 删除文件
func (s *ObjectStorage) RemoveObject(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}
	return nil
}

// GetPresignedURL 生成预签名下载 URL，filename 用于设置下载时的中文文件名
func (s *ObjectStorage) GetPresignedURL(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	reqParams := make(map[string][]string)
	reqParams["response-content-disposition"] = []string{
		fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", filename, url.PathEscape(filename)),
	}
	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("生成下载链接失败: %w", err)
	}
	return presignedURL.String(), nil
}
