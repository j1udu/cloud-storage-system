package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
	"github.com/j1udu/cloud-storage-system/backend/internal/storage"
)

type ShareService struct {
	shareRepo *repository.ShareRepo
	fileRepo  *repository.FileRepo
	storage   *storage.ObjectStorage
}

func NewShareService(shareRepo *repository.ShareRepo, fileRepo *repository.FileRepo, storage *storage.ObjectStorage) *ShareService {
	return &ShareService{shareRepo: shareRepo, fileRepo: fileRepo, storage: storage}
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
	if items == nil {
		items = []model.Share{}
	}

	return &model.ShareListResponse{
		Total: total,
		Items: items,
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

	return &model.PublicShareInfoResponse{Matter: *matter}, nil
}

func (s *ShareService) Download(ctx context.Context, token string, req *model.ShareDownloadRequest) (string, error) {
	share, err := s.validShare(token)
	if err != nil {
		return "", err
	}
	if share.AccessCode != "" && share.AccessCode != req.AccessCode {
		return "", fmt.Errorf("access_code invalid")
	}

	matter, err := s.fileRepo.GetByID(share.MatterID)
	if err != nil {
		return "", fmt.Errorf("matter not found")
	}
	if matter.Status != 1 {
		return "", fmt.Errorf("matter is not active")
	}
	if matter.Dir {
		return "", fmt.Errorf("folder cannot download")
	}

	return s.storage.GetPresignedURL(ctx, matter.StorageKey, matter.Name, time.Hour)
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
