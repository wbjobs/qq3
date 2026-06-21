package handlers

import (
	"clipboard-sync/models"
	"clipboard-sync/services"
	"clipboard-sync/websocket"
	"log"
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
	uid := userID.(uint)

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

	seqID, err := models.GetNextSeqID(uid)
	if err != nil {
		log.Printf("Get seq_id error: %v", err)
		seqID = time.Now().UnixNano()
	}

	needsTrans := h.TranslateService.NeedsTranslation(req.Content)

	item := &models.ClipboardItem{
		UserID:       uid,
		SeqID:        seqID,
		Content:      req.Content,
		Translation:  "",
		SourceDevice: req.DeviceID,
		DeviceName:   req.DeviceName,
		ContentType:  req.ContentType,
		IsTranslated: false,
		CreatedAt:    time.Now(),
	}

	if err := models.CreateClipboardItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存剪贴板内容失败"})
		return
	}

	go h.Hub.SendClipboardSync(uid, websocket.ClipboardSyncData{
		ID:           item.ID,
		SeqID:        item.SeqID,
		Content:      item.Content,
		Translation:  item.Translation,
		DeviceName:   item.DeviceName,
		IsTranslated: item.IsTranslated,
		ContentType:  item.ContentType,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}, req.DeviceID)

	if needsTrans {
		go h.processTranslationAsync(uid, item, req.DeviceID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "同步成功",
		"item":    item,
	})
}

func (h *ClipboardHandler) processTranslationAsync(userID uint, item *models.ClipboardItem, fromDevice string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Translation goroutine panic: %v", r)
		}
	}()

	translation, err := h.TranslateService.TranslateToChinese(item.Content)
	if err != nil {
		log.Printf("Translate error for item %d: %v", item.ID, err)
		return
	}
	if translation == "" {
		return
	}

	item.Translation = translation
	item.IsTranslated = true

	if err := models.DB.Model(item).Updates(map[string]interface{}{
		"translation":   translation,
		"is_translated": true,
	}).Error; err != nil {
		log.Printf("Update translation DB error: %v", err)
		return
	}

	if err := models.PushClipboardToRedis(userID, item); err != nil {
		log.Printf("Update translation Redis error: %v", err)
	}

	go h.Hub.SendTranslationUpdate(userID, websocket.TranslationUpdateData{
		ID:          item.ID,
		SeqID:       item.SeqID,
		Translation: translation,
	})

	log.Printf("Translation completed for item %d, seq=%d", item.ID, item.SeqID)
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
