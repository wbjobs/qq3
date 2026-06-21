package main

import (
	"clipboard-sync/config"
	"clipboard-sync/handlers"
	"clipboard-sync/middleware"
	"clipboard-sync/models"
	"clipboard-sync/websocket"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	models.InitDB(cfg.MySQLDSN)
	models.InitRedis(cfg.RedisAddr, cfg.RedisPwd, cfg.RedisDB)

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
		}
	}

	r.Static("/web", "./web")
	r.Static("/mobile", "./mobile")

	log.Println("Server starting on", cfg.ServerPort)
	log.Fatal(r.Run(cfg.ServerPort))
}
