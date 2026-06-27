package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
	"github.com/j1udu/cloud-storage-system/backend/internal/storage"
)

type ShareService struct {
	shareRepo         *repository.ShareRepo
	fileRepo          *repository.FileRepo
	userRepo          *repository.UserRepo
	storage           *storage.ObjectStorage
	defaultQuotaBytes int64
}

func NewShareService(shareRepo *repository.ShareRepo, fileRepo *repository.FileRepo, userRepo *repository.UserRepo, storage *storage.ObjectStorage, defaultQuotaBytes int64) *ShareService {
	return &ShareService{shareRepo: shareRepo, fileRepo: fileRepo, userRepo: userRepo, storage: storage, defaultQuotaBytes: defaultQuotaBytes}
}

func (s *ShareService) Create(userID uint64, req *model.ShareCreateRequest) (*model.Share, error) {
	matter, err := s.fileRepo.GetByID(req.MatterID)
	if err != nil {
		return nil, fmt.Errorf("matter not found")
	}
	if matter.UserID != userID {
		return nil, fmt.Errorf("no permission")
	}
	if matter.Status != 1 {
		return nil, fmt.Errorf("matter is not active")
	}

	var expireAt *time.Time
	if req.ExpireHour > 0 {
		t := time.Now().UTC().Add(time.Duration(req.ExpireHour) * time.Hour)
		expireAt = &t
	}

	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}

	share := &model.Share{
		UserID:     userID,
		MatterID:   req.MatterID,
		Token:      token,
		AccessCode: req.AccessCode,
		ExpireAt:   expireAt,
		Status:     1,
	}
	if err := s.shareRepo.Create(share); err != nil {
		return nil, err
	}

	return s.shareRepo.GetByID(share.ID)
}

func (s *ShareService) Cancel(userID, shareID uint64) error {
	if err := s.shareRepo.CancelByIDAndUser(shareID, userID); err != nil {
		return fmt.Errorf("share not found")
	}
	return nil
}

func (s *ShareService) List(userID uint64, page, pageSize int) (*model.ShareListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	total, err := s.shareRepo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	items, err := s.shareRepo.ListByUser(userID, offset, pageSize)
	if err != nil {
		return nil, err
	}

	shareItems := make([]model.ShareItem, 0, len(items))
	for _, sh := range items {
		si := model.ShareItem{
			ID:         sh.ID,
			UserID:     sh.UserID,
			MatterID:   sh.MatterID,
			Token:      sh.Token,
			AccessCode: sh.AccessCode,
			ExpireAt:   sh.ExpireAt,
			Status:     sh.Status,
			CreatedAt:  sh.CreatedAt,
			UpdatedAt:  sh.UpdatedAt,
		}
		if m, err := s.fileRepo.GetByID(sh.MatterID); err == nil {
			si.MatterName = m.Name
		}
		shareItems = append(shareItems, si)
	}

	return &model.ShareListResponse{
		Total: total,
		Items: shareItems,
	}, nil
}

func (s *ShareService) GetPublicInfo(token string) (*model.PublicShareInfoResponse, error) {
	share, err := s.validShare(token)
	if err != nil {
		return nil, err
	}

	matter, err := s.fileRepo.GetByID(share.MatterID)
	if err != nil {
		return nil, fmt.Errorf("matter not found")
	}
	if matter.Status != 1 {
		return nil, fmt.Errorf("matter is not active")
	}

	sharerName := ""
	if user, err := s.userRepo.GetByID(share.UserID); err == nil {
		sharerName = user.Nickname
		if sharerName == "" {
			sharerName = user.Username
		}
	}

	return &model.PublicShareInfoResponse{
		Matter:     *matter,
		SharerName: sharerName,
		HasCode:    share.AccessCode != "",
	}, nil
}

func (s *ShareService) DownloadContent(ctx context.Context, token string, req *model.ShareDownloadRequest) (io.ReadCloser, string, int64, string, error) {
	share, err := s.validShare(token)
	if err != nil {
		return nil, "", 0, "", err
	}
	if share.AccessCode != "" && share.AccessCode != req.AccessCode {
		return nil, "", 0, "", fmt.Errorf("access_code invalid")
	}

	matter, err := s.fileRepo.GetByID(share.MatterID)
	if err != nil {
		return nil, "", 0, "", fmt.Errorf("matter not found")
	}
	if matter.Status != 1 {
		return nil, "", 0, "", fmt.Errorf("matter is not active")
	}
	if matter.Dir {
		return nil, "", 0, "", fmt.Errorf("folder cannot download")
	}

	reader, err := s.storage.GetObject(ctx, matter.StorageKey)
	if err != nil {
		return nil, "", 0, "", fmt.Errorf("get file from storage failed: %w", err)
	}

	return reader, matter.Name, matter.Size, matter.MimeType, nil
}

func (s *ShareService) Save(userID uint64, token string, req *model.ShareSaveRequest) (*model.ShareSaveResponse, error) {
	share, err := s.validShare(token)
	if err != nil {
		return nil, err
	}
	if share.AccessCode != "" && share.AccessCode != req.AccessCode {
		return nil, fmt.Errorf("access_code invalid")
	}

	matter, err := s.fileRepo.GetByID(share.MatterID)
	if err != nil {
		return nil, fmt.Errorf("matter not found")
	}
	if matter.Status != 1 {
		return nil, fmt.Errorf("matter is not active")
	}

	if req.ParentID != 0 {
		parent, err := s.fileRepo.GetByID(req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent folder not found")
		}
		if parent.UserID != userID {
			return nil, fmt.Errorf("no permission")
		}
		if !parent.Dir {
			return nil, fmt.Errorf("parent is not a folder")
		}
		if parent.Status != 1 {
			return nil, fmt.Errorf("parent folder is not active")
		}
	}

	exists, err := s.fileRepo.ExistsByName(userID, req.ParentID, matter.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("same name already exists")
	}

	saveBytes, err := s.fileRepo.SumActiveTreeFileSize(matter.UserID, matter.ID)
	if err != nil {
		return nil, err
	}
	usedBytes, err := s.fileRepo.SumUsedBytes(userID)
	if err != nil {
		return nil, err
	}
	if usedBytes+saveBytes > s.defaultQuotaBytes {
		return nil, fmt.Errorf("storage quota exceeded")
	}

	savedMatter, err := s.fileRepo.CopyActiveTree(matter.UserID, matter.ID, userID, req.ParentID)
	if err != nil {
		return nil, err
	}

	return &model.ShareSaveResponse{Matter: *savedMatter}, nil
}

func (s *ShareService) validShare(token string) (*model.Share, error) {
	share, err := s.shareRepo.GetByToken(token)
	if err != nil {
		return nil, fmt.Errorf("share not found")
	}
	if share.Status != 1 {
		return nil, fmt.Errorf("share canceled")
	}
	if share.ExpireAt != nil && share.ExpireAt.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("share expired")
	}
	return share, nil
}

func generateShareToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
