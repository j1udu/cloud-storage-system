package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/hash"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
	"github.com/j1udu/cloud-storage-system/backend/internal/storage"
)

// FileService 文件业务逻辑
type FileService struct {
	repo    *repository.FileRepo
	storage *storage.ObjectStorage
}

// NewFileService 创建 FileService 实例
func NewFileService(repo *repository.FileRepo, storage *storage.ObjectStorage) *FileService {
	return &FileService{repo: repo, storage: storage}
}

// Upload 上传文件：存 MinIO + 写数据库
func (s *FileService) Upload(ctx context.Context, userID uint64, parentID uint64, filename string, fileReader io.Reader, fileSize int64, contentType string) (*model.FileUploadResponse, error) {
	// 计算文件扩展名
	ext := strings.ToLower(filepath.Ext(filename))

	// 计算 MD5（需要同时读取内容算哈希和上传，所以用 TeeReader）
	var buf strings.Builder
	teeReader := io.TeeReader(fileReader, &buf)
	md5Hash, err := hash.MD5FromReader(teeReader)
	if err != nil {
		return nil, fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// storage_key: {user_id}/{md5}{ext}
	storageKey := fmt.Sprintf("%d/%s%s", userID, md5Hash, ext)

	// 把缓冲区的内容和剩余内容拼起来上传
	fullReader := io.MultiReader(strings.NewReader(buf.String()), fileReader)
	if err := s.storage.PutObject(ctx, storageKey, fullReader, fileSize, contentType); err != nil {
		return nil, err
	}

	// 写入数据库
	matter := &model.Matter{
		UserID:     userID,
		ParentID:   parentID,
		Name:       filename,
		Dir:        false,
		Size:       fileSize,
		Ext:        ext,
		MimeType:   contentType,
		MD5:        md5Hash,
		StorageKey: storageKey,
		Status:     1,
	}
	if err := s.repo.Create(matter); err != nil {
		return nil, err
	}

	return &model.FileUploadResponse{
		ID:   matter.ID,
		Name: matter.Name,
		Size: matter.Size,
		Ext:  matter.Ext,
	}, nil
}

// Download 获取文件下载链接
func (s *FileService) Download(ctx context.Context, userID, fileID uint64) (string, error) {
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return "", fmt.Errorf("文件不存在")
	}
	if matter.UserID != userID {
		return "", fmt.Errorf("无权访问此文件")
	}
	if matter.Dir {
		return "", fmt.Errorf("文件夹不能下载")
	}

	url, err := s.storage.GetPresignedURL(ctx, matter.StorageKey, matter.Name, time.Hour)
	if err != nil {
		return "", err
	}
	return url, nil
}

// List 列出文件夹内容
func (s *FileService) List(userID, parentID uint64, page, pageSize int) (*model.FileListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	total, err := s.repo.CountByParent(userID, parentID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListByParent(userID, parentID, offset, pageSize)
	if err != nil {
		return nil, err
	}

	if items == nil {
		items = []model.Matter{}
	}

	return &model.FileListResponse{
		Total: total,
		Items: items,
	}, nil
}

// Delete 软删除文件（移入回收站）
func (s *FileService) Delete(userID, fileID uint64) error {
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("文件不存在")
	}
	if matter.UserID != userID {
		return fmt.Errorf("无权操作此文件")
	}
	return s.repo.UpdateStatus(fileID, 2)
}

// Rename 重命名
func (s *FileService) Rename(userID, fileID uint64, newName string) error {
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("文件不存在")
	}
	if matter.UserID != userID {
		return fmt.Errorf("无权操作此文件")
	}
	return s.repo.UpdateName(fileID, newName)
}
