package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"glata-backend/internal/config"
	"glata-backend/internal/handler"
	"glata-backend/internal/service"
	"glata-backend/pkg/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format); err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}

	// 初始化服务
	chatService := service.NewChatService(cfg)
	
	// 初始化 Agent 存储（使用与聊天服务相同的存储实例）
	service.InitAgentStorage(chatService.GetStorage())
	
	// 初始化处理器
	chatHandler := handler.NewChatHandler(chatService)

	// 创建路由
	router := setupRouter(cfg, chatHandler)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// 启动服务器
	go func() {
		logger.Infof("服务器启动在端口 %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待信号优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("服务器正在关闭...")
	if err := server.Close(); err != nil {
		logger.Errorf("服务器关闭失败: %v", err)
	}
	logger.Info("服务器已关闭")
}

func setupRouter(cfg *config.Config, chatHandler *handler.ChatHandler) *gin.Engine {
	// 设置gin模式
	gin.SetMode(gin.ReleaseMode)
	
	router := gin.New()
	
	// 中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	
	// CORS配置
	corsConfig := cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     cfg.CORS.AllowedMethods,
		AllowHeaders:     cfg.CORS.AllowedHeaders,
		ExposeHeaders:    cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           time.Duration(cfg.CORS.MaxAge) * time.Second,
	}
	router.Use(cors.New(corsConfig))

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	// API路由
	api := router.Group("/api")
	{
		chat := api.Group("/chat")
		{
			chat.POST("/stream", chatHandler.StreamChat)
			chat.POST("/session", chatHandler.CreateSession)
			chat.POST("/session/list", chatHandler.GetSessionList)
			chat.GET("/session/del/:session_id", chatHandler.DeleteSession)
			chat.POST("/session/clear", chatHandler.ClearAllSessions)
			chat.GET("/session/:session_id", chatHandler.GetSession)
			chat.GET("/messages/:session_id", chatHandler.GetMessages)
			chat.PUT("/session/:session_id", chatHandler.UpdateSessionTitle)
			
			// ✅ 新增渲染相关API端点 - 支持会话隔离
			chat.PUT("/message/:message_id/render", chatHandler.UpdateMessageRender)
			chat.PUT("/session/:session_id/render-batch", chatHandler.UpdateSessionRenderBatch)
			chat.GET("/session/:session_id/pending-renders", chatHandler.GetPendingRenders)
		}
	}

	return router
}