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

// CreateFolder 创建文件夹
func (s *FileService) CreateFolder(userID uint64, req *model.FolderCreateRequest) (*model.Matter, error) {
	// 检查同名
	exists, err := s.repo.ExistsByName(userID, req.ParentID, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("同名文件/文件夹已存在")
	}

	folder := &model.Matter{
		UserID:   userID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Dir:      true,
		Status:   1,
	}
	if err := s.repo.Create(folder); err != nil {
		return nil, err
	}

	return s.repo.GetByID(folder.ID)
}

// GetPath 面包屑路径：从当前文件夹往上追溯到根目录
func (s *FileService) GetPath(userID, folderID uint64) ([]model.PathItem, error) {
	if folderID == 0 {
		return []model.PathItem{{ID: 0, Name: "根目录"}}, nil
	}

	// 确认文件夹属于当前用户
	folder, err := s.repo.GetByID(folderID)
	if err != nil {
		return nil, fmt.Errorf("文件夹不存在")
	}
	if folder.UserID != userID {
		return nil, fmt.Errorf("无权访问")
	}

	// 从当前文件夹往上追溯
	var path []model.PathItem
	currentID := folderID
	for currentID != 0 {
		m, err := s.repo.GetByID(currentID)
		if err != nil {
			break
		}
		path = append([]model.PathItem{{ID: m.ID, Name: m.Name}}, path...)
		currentID = m.ParentID
	}
	// 最前面加上根目录
	path = append([]model.PathItem{{ID: 0, Name: "根目录"}}, path...)

	return path, nil
}

// Move 移动文件/文件夹
func (s *FileService) Move(userID, fileID uint64, targetID uint64) error {
	// 校验要移动的文件
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("文件不存在")
	}
	if matter.UserID != userID {
		return fmt.Errorf("无权操作此文件")
	}

	// 不能移到自己里面
	if fileID == targetID {
		return fmt.Errorf("不能移动到自身")
	}

	// targetID != 0 时校验目标文件夹
	if targetID != 0 {
		target, err := s.repo.GetByID(targetID)
		if err != nil {
			return fmt.Errorf("目标文件夹不存在")
		}
		if target.UserID != userID {
			return fmt.Errorf("无权访问目标文件夹")
		}
		if !target.Dir {
			return fmt.Errorf("目标不是文件夹")
		}
	}

	// 检查目标位置有没有同名
	exists, err := s.repo.ExistsByName(userID, targetID, matter.Name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("目标位置存在同名文件")
	}

	return s.repo.UpdateParent(fileID, targetID)
}
