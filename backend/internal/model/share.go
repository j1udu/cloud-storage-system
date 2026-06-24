package model

import "time"

type Share struct {
	ID         uint64     `json:"id"`
	UserID     uint64     `json:"user_id"`
	MatterID   uint64     `json:"matter_id"`
	Token      string     `json:"token"`
	AccessCode string     `json:"access_code"`
	ExpireAt   *time.Time `json:"expire_at"`
	Status     int        `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ShareCreateRequest struct {
	MatterID   uint64 `json:"matter_id" binding:"required"`
	AccessCode string `json:"access_code"`
	ExpireHour int    `json:"expire_hour"`
}

type ShareDownloadRequest struct {
	AccessCode string `json:"access_code"`
}

type ShareSaveRequest struct {
	AccessCode string `json:"access_code"`
	ParentID   uint64 `json:"parent_id"`
}

type ShareItem struct {
	ID         uint64     `json:"id"`
	UserID     uint64     `json:"user_id"`
	MatterID   uint64     `json:"matter_id"`
	MatterName string     `json:"matter_name"`
	Token      string     `json:"token"`
	AccessCode string     `json:"access_code"`
	ExpireAt   *time.Time `json:"expire_at"`
	Status     int        `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ShareListResponse struct {
	Total int64       `json:"total"`
	Items []ShareItem `json:"items"`
}

type PublicShareInfoResponse struct {
	Matter     Matter `json:"matter"`
	SharerName string `json:"sharer_name"`
	HasCode    bool   `json:"has_code"`
}

type ShareSaveResponse struct {
	Matter Matter `json:"matter"`
}
