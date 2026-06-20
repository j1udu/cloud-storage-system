package repository

import (
	"database/sql"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
)

type FileRepo struct {
	db *sql.DB
}

func NewFileRepo(db *sql.DB) *FileRepo {
	return &FileRepo{db: db}
}

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

func (r *FileRepo) GetByID(id uint64) (*model.Matter, error) {
	var m model.Matter
	err := r.db.QueryRow(
		"SELECT id, user_id, parent_id, name, dir, size, ext, mime_type, md5, storage_key, path, status, recycle_root_id, created_at, updated_at FROM matter WHERE id = ?",
		id,
	).Scan(&m.ID, &m.UserID, &m.ParentID, &m.Name, &m.Dir, &m.Size, &m.Ext, &m.MimeType, &m.MD5, &m.StorageKey, &m.Path, &m.Status, &m.RecycleRootID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

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

func (r *FileRepo) CountByParent(userID, parentID uint64) (int64, error) {
	var count int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM matter WHERE user_id = ? AND parent_id = ? AND status = 1",
		userID, parentID,
	).Scan(&count)
	return count, err
}

func (r *FileRepo) ListByStatus(userID uint64, status int, offset, limit int) ([]model.Matter, error) {
	rows, err := r.db.Query(
		"SELECT id, user_id, parent_id, name, dir, size, ext, mime_type, status, created_at, updated_at FROM matter WHERE user_id = ? AND status = ? ORDER BY dir DESC, created_at DESC LIMIT ? OFFSET ?",
		userID, status, limit, offset,
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

func (r *FileRepo) CountByStatus(userID uint64, status int) (int64, error) {
	var count int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM matter WHERE user_id = ? AND status = ?",
		userID, status,
	).Scan(&count)
	return count, err
}

func (r *FileRepo) SumUsedBytes(userID uint64) (int64, error) {
	var usedBytes sql.NullInt64
	err := r.db.QueryRow(
		"SELECT SUM(size) FROM matter WHERE user_id = ? AND dir = 0 AND status IN (1, 2)",
		userID,
	).Scan(&usedBytes)
	if err != nil {
		return 0, err
	}
	if !usedBytes.Valid {
		return 0, nil
	}
	return usedBytes.Int64, nil
}

func (r *FileRepo) ExistsActiveFileByStorageKey(storageKey string) (bool, error) {
	var count int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM matter WHERE storage_key = ? AND dir = 0 AND status = 1",
		storageKey,
	).Scan(&count)
	return count > 0, err
}

func (r *FileRepo) UpdateName(id uint64, name string) error {
	_, err := r.db.Exec("UPDATE matter SET name = ? WHERE id = ?", name, id)
	return err
}

func (r *FileRepo) UpdateStatus(id uint64, status int) error {
	_, err := r.db.Exec("UPDATE matter SET status = ? WHERE id = ?", status, id)
	return err
}

func (r *FileRepo) MoveTreeToRecycle(userID, rootID uint64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids, err := r.listTreeIDs(tx, userID, rootID)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := tx.Exec(
			"UPDATE matter SET status = 2, recycle_root_id = ? WHERE id = ? AND user_id = ? AND status = 1",
			rootID, id, userID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *FileRepo) RestoreTreeFromRecycle(userID, rootID uint64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids, err := r.listTreeIDs(tx, userID, rootID)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := tx.Exec(
			"UPDATE matter SET status = 1, recycle_root_id = 0 WHERE id = ? AND user_id = ? AND status = 2 AND recycle_root_id = ?",
			id, userID, rootID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *FileRepo) UpdateTreeStatus(userID, rootID uint64, fromStatus, toStatus int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids, err := r.listTreeIDs(tx, userID, rootID)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := tx.Exec(
			"UPDATE matter SET status = ?, recycle_root_id = 0 WHERE id = ? AND user_id = ? AND status = ?",
			toStatus, id, userID, fromStatus,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *FileRepo) ListTreeByStatus(userID, rootID uint64, status int) ([]model.Matter, error) {
	pending := []uint64{rootID}
	items := make([]model.Matter, 0)

	for len(pending) > 0 {
		currentID := pending[0]
		pending = pending[1:]

		var current model.Matter
		err := r.db.QueryRow(
			"SELECT id, user_id, parent_id, name, dir, size, ext, mime_type, md5, storage_key, path, status, recycle_root_id, created_at, updated_at FROM matter WHERE id = ? AND user_id = ?",
			currentID, userID,
		).Scan(&current.ID, &current.UserID, &current.ParentID, &current.Name, &current.Dir, &current.Size, &current.Ext, &current.MimeType, &current.MD5, &current.StorageKey, &current.Path, &current.Status, &current.RecycleRootID, &current.CreatedAt, &current.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if current.Status == status {
			items = append(items, current)
		}

		rows, err := r.db.Query(
			"SELECT id FROM matter WHERE user_id = ? AND parent_id = ?",
			userID, currentID,
		)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var childID uint64
			if err := rows.Scan(&childID); err != nil {
				rows.Close()
				return nil, err
			}
			pending = append(pending, childID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return items, nil
}

func (r *FileRepo) ExistsByName(userID, parentID uint64, name string) (bool, error) {
	var count int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM matter WHERE user_id = ? AND parent_id = ? AND name = ? AND status = 1",
		userID, parentID, name,
	).Scan(&count)
	return count > 0, err
}

func (r *FileRepo) GetParentID(id uint64) (uint64, error) {
	var parentID uint64
	err := r.db.QueryRow("SELECT parent_id FROM matter WHERE id = ?", id).Scan(&parentID)
	return parentID, err
}

func (r *FileRepo) UpdateParent(id uint64, parentID uint64) error {
	_, err := r.db.Exec("UPDATE matter SET parent_id = ? WHERE id = ?", parentID, id)
	return err
}

type treeQueryer interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func (r *FileRepo) listTreeIDs(q treeQueryer, userID, rootID uint64) ([]uint64, error) {
	pending := []uint64{rootID}
	ids := make([]uint64, 0, 1)

	for len(pending) > 0 {
		currentID := pending[0]
		pending = pending[1:]
		ids = append(ids, currentID)

		rows, err := q.Query(
			"SELECT id FROM matter WHERE user_id = ? AND parent_id = ?",
			userID, currentID,
		)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var childID uint64
			if err := rows.Scan(&childID); err != nil {
				rows.Close()
				return nil, err
			}
			pending = append(pending, childID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return ids, nil
}
