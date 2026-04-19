package service

import (
	"errors"
	"database/sql"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/hash"
	pkgjwt "github.com/j1udu/cloud-storage-system/backend/internal/pkg/jwt"
	"github.com/j1udu/cloud-storage-system/backend/internal/repository"
)

// UserService 用户业务逻辑，持有 repo 和 JWT 配置
type UserService struct {
	repo          *repository.UserRepo
	jwtSecret     string
	jwtExpireHour int
}

// NewUserService 创建 UserService 实例
func NewUserService(repo *repository.UserRepo, jwtSecret string, jwtExpireHour int) *UserService {
	return &UserService{
		repo:          repo,
		jwtSecret:     jwtSecret,
		jwtExpireHour: jwtExpireHour,
	}
}

// Register 注册
func (s *UserService) Register(req *model.RegisterRequest) (*model.User, error) {
	// 1. 检查用户名是否已存在
	_, err := s.repo.GetByUsername(req.Username)
	if err == nil {
		return nil, errors.New(errcode.GetMsg(errcode.ErrUserExists))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// 2. 加密密码
	hashedPassword, err := hash.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 3. 构造用户对象
	user := &model.User{
		Username: req.Username,
		Password: hashedPassword,
		Nickname: req.Nickname,
	}
	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	// 4. 存入数据库
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	// 5. 查回完整用户信息（含数据库自动生成的字段）
	created, err := s.repo.GetByID(user.ID)
	if err != nil {
		return user, nil
	}

	return created, nil
}

// Login 登录
func (s *UserService) Login(req *model.LoginRequest) (*model.LoginResponse, error) {
	// 1. 查用户
	user, err := s.repo.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errcode.GetMsg(errcode.ErrUserNotFound))
		}
		return nil, err
	}

	// 2. 验证密码
	if !hash.CheckPassword(req.Password, user.Password) {
		return nil, errors.New(errcode.GetMsg(errcode.ErrPasswordWrong))
	}

	// 3. 生成 JWT 令牌
	token, expiresAt, err := pkgjwt.GenerateToken(user.ID, s.jwtSecret, s.jwtExpireHour)
	if err != nil {
		return nil, err
	}

	// 4. 返回令牌和用户信息
	return &model.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      *user,
	}, nil
}

// GetProfile 获取用户信息
func (s *UserService) GetProfile(userID uint64) (*model.User, error) {
	return s.repo.GetByID(userID)
}
