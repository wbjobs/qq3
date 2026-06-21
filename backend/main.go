package main

import (
	"clipboard-sync/config"
	"clipboard-sync/handlers"
	"clipboard-sync/middleware"
	"clipboard-sync/models"
	"clipboard-sync/services"
	"clipboard-sync/websocket"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	models.InitDB(cfg.MySQLDSN)
	models.InitRedis(cfg.RedisAddr, cfg.RedisPwd, cfg.RedisDB)

	initDefaultSensitiveWords()

	hub := websocket.NewHub()
	go hub.Run()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	authHandler := handlers.NewAuthHandler(cfg.JWTSecret)
	clipboardHandler := handlers.NewClipboardHandler(hub)
	deviceHandler := handlers.NewDeviceHandler()
	adminHandler := handlers.NewAdminHandler()

	api := r.Group("/api")
	{
		api.POST("/register", authHandler.Register)
		api.POST("/login", authHandler.Login)

		auth := api.Group("")
		auth.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		{
			auth.POST("/device/bind", deviceHandler.BindDevice)
			auth.GET("/devices", deviceHandler.ListDevices)
			auth.DELETE("/device/:id", deviceHandler.UnbindDevice)

			auth.POST("/clipboard/sync", clipboardHandler.SyncClipboard)
			auth.GET("/clipboard/history", clipboardHandler.GetHistory)

			auth.GET("/ws", func(c *gin.Context) {
				userID, _ := c.Get("user_id")
				deviceID := c.Query("device_id")
				websocket.ServeWS(hub, c.Writer, c.Request, userID.(uint), deviceID)
			})

			auth.GET("/settings", adminHandler.GetUserSettings)
			auth.POST("/settings/silent-mode", adminHandler.SetSilentMode)
			auth.POST("/settings/filter", adminHandler.SetFilterEnable)

			auth.GET("/sensitive-words", adminHandler.ListSensitiveWords)
			auth.POST("/sensitive-words", adminHandler.AddSensitiveWord)
			auth.DELETE("/sensitive-words", adminHandler.RemoveSensitiveWord)
			auth.POST("/sensitive-words/test", adminHandler.TestFilter)
		}
	}

	r.Static("/web", "./web")
	r.Static("/mobile", "./mobile")

	log.Println("Server starting on", cfg.ServerPort)
	log.Fatal(r.Run(cfg.ServerPort))
}

func initDefaultSensitiveWords() {
	svc := services.NewSensitiveWordService()
	defaults := []string{
		"密码",
		"验证码",
		"银行卡密码",
		"支付密码",
		"身份证号",
		"身份证号码",
		"手机号",
		"电话号码",
	}
	for _, word := range defaults {
		if err := svc.AddCustomWord(word); err != nil {
			log.Printf("Init default sensitive word [%s] error: %v", word, err)
		}
	}
	log.Println("Default sensitive words initialized:", len(defaults))
}
