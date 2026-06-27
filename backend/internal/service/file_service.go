package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/hash"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
	"github.com/j1udu/cloud-storage-system/backend/internal/storage"
)

// FileService 文件业务逻辑
type FileService struct {
	repo              *repository.FileRepo
	storage           *storage.ObjectStorage
	defaultQuotaBytes int64
}

// NewFileService 创建 FileService 实例
func NewFileService(repo *repository.FileRepo, storage *storage.ObjectStorage, defaultQuotaBytes int64) *FileService {
	return &FileService{repo: repo, storage: storage, defaultQuotaBytes: defaultQuotaBytes}
}

// Upload 上传文件：存 MinIO + 写数据库
func (s *FileService) Upload(ctx context.Context, userID uint64, parentID uint64, filename string, fileReader io.ReadSeeker, fileSize int64, contentType string) (*model.FileUploadResponse, error) {
	usedBytes, err := s.repo.SumUsedBytes(userID)
	if err != nil {
		return nil, err
	}
	if usedBytes+fileSize > s.defaultQuotaBytes {
		return nil, fmt.Errorf("storage quota exceeded")
	}

	// 计算文件扩展名
	ext := strings.ToLower(filepath.Ext(filename))

	md5Hash, err := hash.MD5FromReader(fileReader)
	if err != nil {
		return nil, fmt.Errorf("计算文件哈希失败: %w", err)
	}
	if _, err := fileReader.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("重置上传文件读取位置失败: %w", err)
	}

	// storage_key: {user_id}/{md5}{ext}
	storageKey := fmt.Sprintf("%d/%s%s", userID, md5Hash, ext)

	if err := s.storage.PutObject(ctx, storageKey, fileReader, fileSize, contentType); err != nil {
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

// DownloadContent 获取文件内容流，返回值：reader, filename, size, mimeType, error
func (s *FileService) DownloadContent(ctx context.Context, userID, fileID uint64) (io.ReadCloser, string, int64, string, error) {
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return nil, "", 0, "", fmt.Errorf("文件不存在")
	}
	if matter.UserID != userID {
		return nil, "", 0, "", fmt.Errorf("无权访问此文件")
	}
	if matter.Status != 1 {
		return nil, "", 0, "", fmt.Errorf("文件已被删除")
	}
	if matter.Dir {
		return nil, "", 0, "", fmt.Errorf("文件夹不能下载")
	}

	reader, err := s.storage.GetObject(ctx, matter.StorageKey)
	if err != nil {
		return nil, "", 0, "", fmt.Errorf("get file from storage failed: %w", err)
	}

	return reader, matter.Name, matter.Size, matter.MimeType, nil
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

// ListRecycle lists files and folders in recycle bin.
func (s *FileService) ListRecycle(userID uint64, page, pageSize int) (*model.FileListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	total, err := s.repo.CountByStatus(userID, 2)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListByStatus(userID, 2, offset, pageSize)
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
	if matter.Status != 1 {
		return fmt.Errorf("文件已被删除")
	}
	return s.repo.MoveTreeToRecycle(userID, fileID)
}

// Restore restores a file or folder from recycle bin.
func (s *FileService) Restore(userID, fileID uint64) error {
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found")
	}
	if matter.UserID != userID {
		return fmt.Errorf("no permission")
	}
	if matter.Status != 2 {
		return fmt.Errorf("file is not in recycle bin")
	}

	if matter.ParentID != 0 {
		parent, err := s.repo.GetByID(matter.ParentID)
		if err != nil {
			return fmt.Errorf("parent folder not found")
		}
		if parent.UserID != userID {
			return fmt.Errorf("no permission")
		}
		if !parent.Dir {
			return fmt.Errorf("parent is not a folder")
		}
		if parent.Status != 1 {
			return fmt.Errorf("parent folder is not active")
		}
	}

	exists, err := s.repo.ExistsByName(userID, matter.ParentID, matter.Name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("same name already exists")
	}

	return s.repo.RestoreTreeFromRecycle(userID, fileID)
}

// PermanentDelete marks a recycled file or folder as deleted.
func (s *FileService) PermanentDelete(ctx context.Context, userID, fileID uint64) error {
	matter, err := s.repo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found")
	}
	if matter.UserID != userID {
		return fmt.Errorf("no permission")
	}
	if matter.Status != 2 {
		return fmt.Errorf("file is not in recycle bin")
	}

	items, err := s.repo.ListTreeByStatus(userID, fileID, 2)
	if err != nil {
		return err
	}

	deletedKeys := make(map[string]struct{})
	for _, item := range items {
		if item.Dir {
			continue
		}
		if item.StorageKey == "" {
			continue
		}
		if _, ok := deletedKeys[item.StorageKey]; ok {
			continue
		}
		hasActiveRef, err := s.repo.ExistsActiveFileByStorageKey(item.StorageKey)
		if err != nil {
			return err
		}
		if hasActiveRef {
			continue
		}
		if err := s.storage.RemoveObject(ctx, item.StorageKey); err != nil {
			return err
		}
		deletedKeys[item.StorageKey] = struct{}{}
	}

	return s.repo.UpdateTreeStatus(userID, fileID, 2, 3)
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
	if matter.Status != 1 {
		return fmt.Errorf("文件已被删除")
	}
	return s.repo.UpdateName(fileID, newName)
}

// CreateFolder 创建文件夹
func (s *FileService) CreateFolder(userID uint64, req *model.FolderCreateRequest) (*model.Matter, error) {
	// 校验父目录有效性
	if req.ParentID != 0 {
		parent, err := s.repo.GetByID(req.ParentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("父目录不存在")
			}
			return nil, err
		}
		if parent.UserID != userID {
			return nil, fmt.Errorf("无权访问父目录")
		}
		if !parent.Dir {
			return nil, fmt.Errorf("父目标不是文件夹")
		}
	}

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
	if !folder.Dir {
		return nil, fmt.Errorf("目标不是文件夹")
	}

	// 从当前文件夹往上追溯，先按从当前到根的顺序追加，最后反转
	path := []model.PathItem{{ID: folder.ID, Name: folder.Name}}
	currentID := folder.ParentID
	for currentID != 0 {
		m, err := s.repo.GetByID(currentID)
		if err != nil {
			return nil, fmt.Errorf("路径数据异常")
		}
		path = append(path, model.PathItem{ID: m.ID, Name: m.Name})
		currentID = m.ParentID
	}
	// 反转：从根到当前
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
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
	if matter.Status != 1 {
		return fmt.Errorf("文件已被删除")
	}

	// 不能移到自己里面
	if fileID == targetID {
		return fmt.Errorf("不能移动到自身")
	}

	// 不能把文件夹移到自己的子孙目录（会形成循环引用）
	if matter.Dir && targetID != 0 {
		ancestorID := targetID
		for ancestorID != 0 {
			if ancestorID == fileID {
				return fmt.Errorf("不能移动到自身的子目录")
			}
			ancestor, err := s.repo.GetByID(ancestorID)
			if err != nil {
				return fmt.Errorf("路径数据异常")
			}
			ancestorID = ancestor.ParentID
		}
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
