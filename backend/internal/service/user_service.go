package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/j1udu/cloud-storage-system/backend/internal/cache"
	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/hash"
	pkgjwt "github.com/j1udu/cloud-storage-system/backend/internal/pkg/jwt"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
)

// UserService 用户业务逻辑
type UserService struct {
	repo          *repository.UserRepo
	userCache     *cache.UserCache
	sessionCache  *cache.SessionCache
	jwtSecret     string
	jwtExpireHour int
}

// NewUserService 创建 UserService 实例
func NewUserService(repo *repository.UserRepo, userCache *cache.UserCache, sessionCache *cache.SessionCache, jwtSecret string, jwtExpireHour int) *UserService {
	return &UserService{
		repo:          repo,
		userCache:     userCache,
		sessionCache:  sessionCache,
		jwtSecret:     jwtSecret,
		jwtExpireHour: jwtExpireHour,
	}
}

// Register 注册
func (s *UserService) Register(req *model.RegisterRequest) (*model.User, error) {
	_, err := s.repo.GetByUsername(req.Username)
	if err == nil {
		return nil, errors.New(errcode.GetMsg(errcode.ErrUserExists))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	hashedPassword, err := hash.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: req.Username,
		Password: hashedPassword,
		Nickname: req.Nickname,
	}
	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	created, err := s.repo.GetByID(user.ID)
	if err != nil {
		return user, nil
	}
	return created, nil
}

// Login 登录
func (s *UserService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	user, err := s.repo.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errcode.GetMsg(errcode.ErrUserNotFound))
		}
		return nil, err
	}

	if !hash.CheckPassword(req.Password, user.Password) {
		return nil, errors.New(errcode.GetMsg(errcode.ErrPasswordWrong))
	}

	token, expiresAt, err := pkgjwt.GenerateToken(user.ID, s.jwtSecret, s.jwtExpireHour)
	if err != nil {
		return nil, err
	}

	// 登录成功，写入会话到 Redis
	_ = s.sessionCache.Set(ctx, user.ID, token, expiresAt)

	return &model.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      *user,
	}, nil
}

// GetProfile 获取用户信息（先查 Redis 缓存，未命中查数据库）
func (s *UserService) GetProfile(ctx context.Context, userID uint64) (*model.User, error) {
	// 1. 查缓存
	cached, err := s.userCache.Get(ctx, userID)
	if err == nil && cached != nil {
		status, _ := strconv.Atoi(cached["status"])
		createdAt, _ := time.Parse("2006-01-02T15:04:05.999Z07:00", cached["created_at"])
		updatedAt, _ := time.Parse("2006-01-02T15:04:05.999Z07:00", cached["updated_at"])
		return &model.User{
			ID:        userID,
			Username:  cached["username"],
			Nickname:  cached["nickname"],
			Status:    status,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}, nil
	}

	// 2. 缓存未命中，查数据库
	user, err := s.repo.GetByID(userID)
	if err != nil {
		return nil, err
	}

	// 3. 写入缓存
	_ = s.userCache.Set(ctx, userID, map[string]interface{}{
		"username":   user.Username,
		"nickname":   user.Nickname,
		"status":     fmt.Sprintf("%d", user.Status),
		"created_at": user.CreatedAt.Format("2006-01-02T15:04:05.999Z07:00"),
		"updated_at": user.UpdatedAt.Format("2006-01-02T15:04:05.999Z07:00"),
	})

	return user, nil
}

// Logout 登出（删除 Redis 会话，使 token 失效）
func (s *UserService) Logout(ctx context.Context, userID uint64) error {
	return s.sessionCache.Delete(ctx, userID)
}
