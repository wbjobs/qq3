package handlers

import (
	"clipboard-sync/models"
	"clipboard-sync/services"
	"clipboard-sync/websocket"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ClipboardHandler struct {
	Hub              *websocket.Hub
	TranslateService *services.TranslateService
}

func NewClipboardHandler(hub *websocket.Hub) *ClipboardHandler {
	return &ClipboardHandler{
		Hub:              hub,
		TranslateService: services.NewTranslateService(),
	}
}

type SyncClipboardRequest struct {
	Content      string `json:"content" binding:"required"`
	DeviceID     string `json:"device_id"`
	DeviceName   string `json:"device_name"`
	ContentType  string `json:"content_type"`
}

func (h *ClipboardHandler) SyncClipboard(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req SyncClipboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ContentType == "" {
		req.ContentType = "text"
	}
	if req.DeviceName == "" {
		req.DeviceName = "未知设备"
	}

	translation, needsTrans, _ := h.TranslateService.ProcessClipboardContent(req.Content)

	item := &models.ClipboardItem{
		UserID:       userID.(uint),
		Content:      req.Content,
		Translation:  translation,
		SourceDevice: req.DeviceID,
		DeviceName:   req.DeviceName,
		ContentType:  req.ContentType,
		IsTranslated: needsTrans,
		CreatedAt:    time.Now(),
	}

	if err := models.CreateClipboardItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存剪贴板内容失败"})
		return
	}

	go h.Hub.SendClipboardSync(userID.(uint), websocket.ClipboardSyncData{
		ID:           item.ID,
		Content:      item.Content,
		Translation:  item.Translation,
		DeviceName:   item.DeviceName,
		IsTranslated: item.IsTranslated,
		ContentType:  item.ContentType,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}, req.DeviceID)

	c.JSON(http.StatusOK, gin.H{
		"message": "同步成功",
		"item":    item,
	})
}

func (h *ClipboardHandler) GetHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, total, err := models.GetClipboardHistory(userID.(uint), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"limit": limit,
		"offset": offset,
	})
}
