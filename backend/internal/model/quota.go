package model

type StorageQuotaResponse struct {
	UsedBytes      int64 `json:"used_bytes"`
	QuotaBytes     int64 `json:"quota_bytes"`
	AvailableBytes int64 `json:"available_bytes"`
}
