package server

import (
	"MPHEDev/pkg/core/participant/utils"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MessageHandler 消息处理接口
type MessageHandler interface {
	HandleMessage(fromID int, message string)
	GetOnlineParticipants() map[int]string
	GetID() int
	GetDataSplit() string
}

// HTTPServer HTTP服务器
type HTTPServer struct {
	Server         *http.Server
	Port           int
	LocalIP        string
	Handlers       map[string]http.HandlerFunc
	MessageHandler MessageHandler // 使用接口
}

// NewHTTPServer 创建新的HTTP服务器
func NewHTTPServer(port int, handlers map[string]http.HandlerFunc, messageHandler MessageHandler) *HTTPServer {
	// 获取本机IP
	localIP, err := utils.GetLocalIP()
	if err != nil {
		fmt.Printf("警告: 获取本机IP失败: %v\n", err)
		localIP = "未知"
	}

	// 创建Gin路由器
	router := gin.Default()

	// 添加状态页面
	router.GET("/status", func(c *gin.Context) {
		onlineParticipants := messageHandler.GetOnlineParticipants()
		c.JSON(http.StatusOK, gin.H{
			"id":           messageHandler.GetID(),
			"ip":           localIP,
			"port":         port,
			"status":       "online",
			"data_split":   messageHandler.GetDataSplit(),
			"participants": onlineParticipants,
		})
	})

	// 添加消息处理接口
	router.POST("/message", func(c *gin.Context) {
		var req struct {
			From    int    `json:"from"`
			Message string `json:"message"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求"})
			return
		}

		// 处理消息
		messageHandler.HandleMessage(req.From, req.Message)
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	})

	// 添加其他处理器
	for path, handler := range handlers {
		router.Any(path, gin.WrapF(handler))
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	return &HTTPServer{
		Server:         server,
		Port:           port,
		LocalIP:        localIP,
		Handlers:       handlers,
		MessageHandler: messageHandler,
	}
}

// Start 启动HTTP服务器
func (hs *HTTPServer) Start() error {
	fmt.Printf("参与方HTTP服务器启动中...\n")
	fmt.Printf("本机IP: %s\n", hs.LocalIP)
	fmt.Printf("监听地址: 0.0.0.0:%d\n", hs.Port)
	fmt.Printf("状态页面: http://%s:%d/status\n", hs.LocalIP, hs.Port)
	fmt.Printf("等待连接...\n\n")

	// 在后台启动HTTP服务器，不阻塞主线程
	go func() {
		if err := hs.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP服务器错误: %v\n", err)
		}
	}()

	return nil
}

// Stop 停止HTTP服务器
func (hs *HTTPServer) Stop() error {
	// 这里可以实现优雅关闭逻辑
	return nil
}

// GetLocalIP 获取本机IP地址
func (hs *HTTPServer) GetLocalIP() string {
	return hs.LocalIP
}
