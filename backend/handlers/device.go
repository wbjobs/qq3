package handlers

import (
	"clipboard-sync/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type DeviceHandler struct{}

func NewDeviceHandler() *DeviceHandler {
	return &DeviceHandler{}
}

type BindDeviceRequest struct {
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
	DeviceType string `json:"device_type" binding:"required,oneof=web mobile desktop"`
}

func (h *DeviceHandler) BindDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req BindDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingDevice models.Device
	result := models.DB.Where("device_id = ?", req.DeviceID).First(&existingDevice)

	if result.Error == nil {
		if existingDevice.UserID != userID.(uint) {
			c.JSON(http.StatusConflict, gin.H{"error": "该设备已绑定到其他用户"})
			return
		}
		existingDevice.DeviceName = req.DeviceName
		existingDevice.DeviceType = req.DeviceType
		existingDevice.LastIP = c.ClientIP()
		existingDevice.LastSeen = time.Now()
		models.DB.Save(&existingDevice)
		c.JSON(http.StatusOK, gin.H{"message": "设备更新成功", "device": existingDevice})
		return
	}

	device := models.Device{
		UserID:     userID.(uint),
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		DeviceType: req.DeviceType,
		LastIP:     c.ClientIP(),
		LastSeen:   time.Now(),
	}

	if err := models.DB.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设备绑定失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "设备绑定成功", "device": device})
}

func (h *DeviceHandler) ListDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var devices []models.Device
	if err := models.DB.Where("user_id = ?", userID.(uint)).
		Order("last_seen DESC").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

func (h *DeviceHandler) UnbindDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")

	result := models.DB.Where("id = ? AND user_id = ?", deviceID, userID.(uint)).
		Delete(&models.Device{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解绑设备失败"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设备解绑成功"})
}
