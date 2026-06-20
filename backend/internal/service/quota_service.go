package service

import (
	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
)

type QuotaService struct {
	fileRepo     *repository.FileRepo
	defaultBytes int64
}

func NewQuotaService(fileRepo *repository.FileRepo, defaultBytes int64) *QuotaService {
	return &QuotaService{fileRepo: fileRepo, defaultBytes: defaultBytes}
}

func (s *QuotaService) Get(userID uint64) (*model.StorageQuotaResponse, error) {
	usedBytes, err := s.fileRepo.SumUsedBytes(userID)
	if err != nil {
		return nil, err
	}

	return &model.StorageQuotaResponse{
		UsedBytes:      usedBytes,
		QuotaBytes:     s.defaultBytes,
		AvailableBytes: s.defaultBytes - usedBytes,
	}, nil
}
