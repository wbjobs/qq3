package handlers

import (
	"clipboard-sync/models"
	"clipboard-sync/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	SensitiveService *services.SensitiveWordService
}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{
		SensitiveService: services.NewSensitiveWordService(),
	}
}

type AddSensitiveWordRequest struct {
	Word string `json:"word" binding:"required,min=1,max=50"`
}

type RemoveSensitiveWordRequest struct {
	Word string `json:"word" binding:"required"`
}

type SetSilentModeRequest struct {
	Enabled bool `json:"enabled" binding:"required"`
}

type SetFilterRequest struct {
	Enabled bool `json:"enabled" binding:"required"`
}

func (h *AdminHandler) GetUserSettings(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	settings, err := models.GetUserSettings(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户设置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
	})
}

func (h *AdminHandler) SetSilentMode(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var req SetSilentModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := models.SetSilentMode(uid, req.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置静默模式失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "静默模式已更新",
		"settings": settings,
	})
}

func (h *AdminHandler) SetFilterEnable(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var req SetFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := models.SetFilterEnable(uid, req.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置过滤开关失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "敏感词过滤已更新",
		"settings": settings,
	})
}

func (h *AdminHandler) ListSensitiveWords(c *gin.Context) {
	words, err := h.SensitiveService.ListCustomWords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取敏感词列表失败"})
		return
	}

	if words == nil {
		words = []string{}
	}

	defaultPatterns := map[string]string{
		"phone":    "中国手机号 (1[3-9]XXXXXXXXX)",
		"idcard":   "18位身份证号",
		"email":    "邮箱地址",
		"bankcard": "银行卡号 (15-19位)",
	}

	c.JSON(http.StatusOK, gin.H{
		"words":            words,
		"custom_words":     words,
		"default_patterns": defaultPatterns,
		"total_custom":     len(words),
	})
}

func (h *AdminHandler) AddSensitiveWord(c *gin.Context) {
	userID, _ := c.Get("user_id")
	_ = userID

	var req AddSensitiveWordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	words, _ := h.SensitiveService.ListCustomWords()
	if len(words) >= 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "自定义敏感词数量已达上限 (200)"})
		return
	}

	if err := h.SensitiveService.AddCustomWord(req.Word); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加敏感词失败"})
		return
	}

	h.SensitiveService.RefreshCache()

	c.JSON(http.StatusOK, gin.H{
		"message": "敏感词添加成功",
		"word":    req.Word,
	})
}

func (h *AdminHandler) RemoveSensitiveWord(c *gin.Context) {
	var req RemoveSensitiveWordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.SensitiveService.RemoveCustomWord(req.Word); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除敏感词失败"})
		return
	}

	h.SensitiveService.RefreshCache()

	c.JSON(http.StatusOK, gin.H{
		"message": "敏感词删除成功",
		"word":    req.Word,
	})
}

func (h *AdminHandler) TestFilter(c *gin.Context) {
	type TestRequest struct {
		Text string `json:"text" binding:"required"`
	}
	var req TestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filtered, hits, err := h.SensitiveService.FilterText(req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "过滤测试失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"original":   req.Text,
		"filtered":   filtered,
		"hits":       hits,
		"hit_count":  len(hits),
		"is_filtered": len(hits) > 0,
	})
}
