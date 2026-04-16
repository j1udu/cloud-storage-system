package repository

import (
	"database/sql"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
)

// UserRepo 用户数据访问，持有数据库连接池
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo 创建 UserRepo 实例
func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create 插入新用户
func (r *UserRepo) Create(user *model.User) error {
	_, err := r.db.Exec(
		"INSERT INTO users (username, password, nickname) VALUES (?, ?, ?)",
		user.Username, user.Password, user.Nickname,
	)
	return err
}

// GetByUsername 按用户名查询（登录时用，包含密码字段）
func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.QueryRow(
		"SELECT id, username, password, nickname, status, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Password, &user.Nickname, &user.Status, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByID 按ID查询（获取用户信息时用，不包含密码）
func (r *UserRepo) GetByID(id uint64) (*model.User, error) {
	var user model.User
	err := r.db.QueryRow(
		"SELECT id, username, nickname, status, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Nickname, &user.Status, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
