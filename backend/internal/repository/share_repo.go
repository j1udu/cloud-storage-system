package repository

import (
	"database/sql"

	"github.com/j1udu/cloud-storage-system/backend/internal/model"
)

type ShareRepo struct {
	db *sql.DB
}

func NewShareRepo(db *sql.DB) *ShareRepo {
	return &ShareRepo{db: db}
}

func (r *ShareRepo) Create(share *model.Share) error {
	result, err := r.db.Exec(
		"INSERT INTO shares (user_id, matter_id, token, access_code, expire_at, status) VALUES (?, ?, ?, ?, ?, ?)",
		share.UserID, share.MatterID, share.Token, share.AccessCode, share.ExpireAt, share.Status,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	share.ID = uint64(id)
	return nil
}

func (r *ShareRepo) GetByID(id uint64) (*model.Share, error) {
	var share model.Share
	var expireAt sql.NullTime
	err := r.db.QueryRow(
		"SELECT id, user_id, matter_id, token, access_code, expire_at, status, created_at, updated_at FROM shares WHERE id = ?",
		id,
	).Scan(&share.ID, &share.UserID, &share.MatterID, &share.Token, &share.AccessCode, &expireAt, &share.Status, &share.CreatedAt, &share.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if expireAt.Valid {
		t := expireAt.Time
		share.ExpireAt = &t
	}
	return &share, nil
}

func (r *ShareRepo) GetByToken(token string) (*model.Share, error) {
	var share model.Share
	var expireAt sql.NullTime
	err := r.db.QueryRow(
		"SELECT id, user_id, matter_id, token, access_code, expire_at, status, created_at, updated_at FROM shares WHERE token = ?",
		token,
	).Scan(&share.ID, &share.UserID, &share.MatterID, &share.Token, &share.AccessCode, &expireAt, &share.Status, &share.CreatedAt, &share.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if expireAt.Valid {
		t := expireAt.Time
		share.ExpireAt = &t
	}
	return &share, nil
}

func (r *ShareRepo) ListByUser(userID uint64, offset, limit int) ([]model.Share, error) {
	rows, err := r.db.Query(
		"SELECT id, user_id, matter_id, token, access_code, expire_at, status, created_at, updated_at FROM shares WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Share
	for rows.Next() {
		var share model.Share
		var expireAt sql.NullTime
		if err := rows.Scan(&share.ID, &share.UserID, &share.MatterID, &share.Token, &share.AccessCode, &expireAt, &share.Status, &share.CreatedAt, &share.UpdatedAt); err != nil {
			return nil, err
		}
		if expireAt.Valid {
			t := expireAt.Time
			share.ExpireAt = &t
		}
		items = append(items, share)
	}
	return items, nil
}

func (r *ShareRepo) CountByUser(userID uint64) (int64, error) {
	var count int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM shares WHERE user_id = ?",
		userID,
	).Scan(&count)
	return count, err
}

func (r *ShareRepo) CancelByIDAndUser(id, userID uint64) error {
	result, err := r.db.Exec(
		"UPDATE shares SET status = 2 WHERE id = ? AND user_id = ?",
		id, userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
