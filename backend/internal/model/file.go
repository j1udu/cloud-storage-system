package model

import "time"

// Matter 对应数据库 matter 表，文件和文件夹共用
type Matter struct {
	ID         uint64    `json:"id"`
	UserID     uint64    `json:"user_id"`
	ParentID   uint64    `json:"parent_id"`
	Name       string    `json:"name"`
	Dir        bool      `json:"dir"`        // true=文件夹 false=文件
	Size       int64     `json:"size"`       // 文件大小(字节)，文件夹为0
	Ext        string    `json:"ext"`        // 扩展名，如 ".pdf"
	MimeType   string    `json:"mime_type"`
	MD5        string    `json:"md5"`
	StorageKey string    `json:"-"`          // MinIO 对象键，不暴露给前端
	Path       string    `json:"path"`       // 物化路径
	Status     int       `json:"status"`     // 1=正常 2=回收站 3=已删除
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// FileUploadResponse 上传成功后的响应
type FileUploadResponse struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Ext  string `json:"ext"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	Total   int64     `json:"total"`
	Items   []Matter  `json:"items"`
}
