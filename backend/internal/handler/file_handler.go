package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/j1udu/cloud-storage-system/backend/internal/model"
	"github.com/j1udu/cloud-storage-system/backend/internal/pkg/errcode"
	"github.com/j1udu/cloud-storage-system/backend/internal/service"
)

// FileHandler handles file APIs.
type FileHandler struct {
	fileService *service.FileService
}

// NewFileHandler creates a FileHandler.
func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

// Upload uploads a file.
func (h *FileHandler) Upload(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "请选择要上传的文件")
		return
	}
	defer file.Close()

	parentIDStr := c.DefaultPostForm("parent_id", "0")
	parentID, _ := strconv.ParseUint(parentIDStr, 10, 64)

	resp, err := h.fileService.Upload(c.Request.Context(), userID.(uint64), parentID, header.Filename, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, resp)
}

// Download returns a download URL.
func (h *FileHandler) Download(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "无效的文件ID")
		return
	}

	url, err := h.fileService.Download(c.Request.Context(), userID.(uint64), fileID)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, gin.H{"url": url})
}

// List lists active files and folders.
func (h *FileHandler) List(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	folderIDStr := c.DefaultQuery("folder_id", "0")
	folderID, _ := strconv.ParseUint(folderIDStr, 10, 64)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.fileService.List(userID.(uint64), folderID, page, pageSize)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, resp)
}

// ListRecycle lists recycle bin items.
func (h *FileHandler) ListRecycle(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "invalid user id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.fileService.ListRecycle(userID.(uint64), page, pageSize)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, resp)
}

// Delete moves a file or folder to recycle bin.
func (h *FileHandler) Delete(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "无效的文件ID")
		return
	}

	if err := h.fileService.Delete(userID.(uint64), fileID); err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, nil)
}

// Restore restores a file or folder from recycle bin.
func (h *FileHandler) Restore(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "无效的文件ID")
		return
	}

	if err := h.fileService.Restore(userID.(uint64), fileID); err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, nil)
}

// PermanentDelete marks a recycled file or folder as deleted.
func (h *FileHandler) PermanentDelete(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "invalid user id")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "invalid file id")
		return
	}

	if err := h.fileService.PermanentDelete(c.Request.Context(), userID.(uint64), fileID); err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, nil)
}

// Rename renames a file or folder.
func (h *FileHandler) Rename(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "无效的文件ID")
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "请输入新名称")
		return
	}

	if err := h.fileService.Rename(userID.(uint64), fileID, req.Name); err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, nil)
}

// CreateFolder creates a folder.
func (h *FileHandler) CreateFolder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	var req model.FolderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "参数错误")
		return
	}

	folder, err := h.fileService.CreateFolder(userID.(uint64), &req)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, folder)
}

// GetPath returns folder path.
func (h *FileHandler) GetPath(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	folderIDStr := c.DefaultQuery("folder_id", "0")
	folderID, err := strconv.ParseUint(folderIDStr, 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "无效的文件夹ID")
		return
	}

	path, err := h.fileService.GetPath(userID.(uint64), folderID)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, path)
}

// Move moves a file or folder.
func (h *FileHandler) Move(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, errcode.ErrParamInvalid, "无效的文件ID")
		return
	}

	var req model.MoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, errcode.ErrParamInvalid, "参数错误")
		return
	}

	if err := h.fileService.Move(userID.(uint64), fileID, req.TargetID); err != nil {
		Fail(c, errcode.ErrParamInvalid, err.Error())
		return
	}

	Success(c, nil)
}
