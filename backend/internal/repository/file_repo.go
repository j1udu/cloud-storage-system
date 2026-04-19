package repository

import (
	"database/sql"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
)

// FileRepo 文件数据访问
type FileRepo struct {
	db *sql.DB
}

// NewFileRepo 创建 FileRepo 实例
func NewFileRepo(db *sql.DB) *FileRepo {
	return &FileRepo{db: db}
}

// Create 插入一条文件/文件夹记录，回填自增ID
func (r *FileRepo) Create(m *model.Matter) error {
	result, err := r.db.Exec(
		"INSERT INTO matter (user_id, parent_id, name, dir, size, ext, mime_type, md5, storage_key, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.UserID, m.ParentID, m.Name, m.Dir, m.Size, m.Ext, m.MimeType, m.MD5, m.StorageKey, m.Status,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	m.ID = uint64(id)
	return nil
}

// GetByID 按ID查询
func (r *FileRepo) GetByID(id uint64) (*model.Matter, error) {
	var m model.Matter
	err := r.db.QueryRow(
		"SELECT id, user_id, parent_id, name, dir, size, ext, mime_type, md5, storage_key, path, status, created_at, updated_at FROM matter WHERE id = ?",
		id,
	).Scan(&m.ID, &m.UserID, &m.ParentID, &m.Name, &m.Dir, &m.Size, &m.Ext, &m.MimeType, &m.MD5, &m.StorageKey, &m.Path, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListByParent 查询某用户某文件夹下的文件列表（分页）
func (r *FileRepo) ListByParent(userID, parentID uint64, offset, limit int) ([]model.Matter, error) {
	rows, err := r.db.Query(
		"SELECT id, user_id, parent_id, name, dir, size, ext, mime_type, status, created_at, updated_at FROM matter WHERE user_id = ? AND parent_id = ? AND status = 1 ORDER BY dir DESC, created_at DESC LIMIT ? OFFSET ?",
		userID, parentID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Matter
	for rows.Next() {
		var m model.Matter
		if err := rows.Scan(&m.ID, &m.UserID, &m.ParentID, &m.Name, &m.Dir, &m.Size, &m.Ext, &m.MimeType, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, nil
}

// CountByParent 统计某用户某文件夹下的文件数量
func (r *FileRepo) CountByParent(userID, parentID uint64) (int64, error) {
	var count int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM matter WHERE user_id = ? AND parent_id = ? AND status = 1",
		userID, parentID,
	).Scan(&count)
	return count, err
}

// UpdateName 重命名
func (r *FileRepo) UpdateName(id uint64, name string) error {
	_, err := r.db.Exec("UPDATE matter SET name = ? WHERE id = ?", name, id)
	return err
}

// UpdateStatus 更改状态（软删除/恢复）
func (r *FileRepo) UpdateStatus(id uint64, status int) error {
	_, err := r.db.Exec("UPDATE matter SET status = ? WHERE id = ?", status, id)
	return err
}
