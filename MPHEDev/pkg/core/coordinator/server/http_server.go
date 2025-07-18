package server

import (
	"MPHEDev/pkg/core/coordinator/utils"
	"fmt"

	"github.com/gin-gonic/gin"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	//Gin框架的路由引擎，可通过Router.POST()等方法注册API路由和处理函数
	Router *gin.Engine
	//HTTP服务器监听的端口号，等待参与方HTTP请求
	Port string
	//本机IP地址
	LocalIP string
}

// NewHTTPServer 创建新的HTTP服务器
func NewHTTPServer(port string) *HTTPServer {
	// 获取本机IP
	localIP, err := utils.GetLocalIP()
	if err != nil {
		fmt.Printf("警告: 获取本机IP失败: %v\n", err)
		localIP = "未知"
	}

	// 使用gin.New()而不是gin.Default()，避免默认中间件冲突
	router := gin.New()
	// 添加Recovery中间件（gin.Default()包含的中间件）
	router.Use(gin.Recovery())

	// 添加CORS中间件
	router.Use(corsMiddleware())

	return &HTTPServer{
		Router:  router,
		Port:    port,
		LocalIP: localIP,
	}
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许所有来源
		c.Header("Access-Control-Allow-Origin", "*")
		// 允许的请求方法
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// 允许的请求头
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, X-Requested-With")
		// 允许发送凭证
		c.Header("Access-Control-Allow-Credentials", "true")
		// 预检请求的缓存时间
		c.Header("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Start 启动HTTP服务器
func (hs *HTTPServer) Start() error {
	fmt.Printf("协调器启动中...\n")
	fmt.Printf("本机IP: %s\n", hs.LocalIP)
	fmt.Printf("监听地址: 0.0.0.0:%s\n", hs.Port)
	fmt.Printf("详细状态页面: http://%s:%s/status\n", hs.LocalIP, hs.Port)
	fmt.Printf("在线状态页面: http://%s:%s/status/online\n", hs.LocalIP, hs.Port)
	fmt.Printf("等待参与方连接...\n\n")

	// 设置HTTP服务器超时配置
	hs.Router.Use(func(c *gin.Context) {
		// 设置请求超时时间为5分钟
		c.Request.Header.Set("Connection", "keep-alive")
		c.Next()
	})

	return hs.Router.Run(":" + hs.Port)
}

// Stop 停止HTTP服务器
func (hs *HTTPServer) Stop() error {
	// 这里可以实现优雅关闭逻辑
	return nil
}

// GetRouter 获取路由器
func (hs *HTTPServer) GetRouter() *gin.Engine {
	return hs.Router
}

// GetLocalIP 获取本机IP地址
func (hs *HTTPServer) GetLocalIP() string {
	return hs.LocalIP
}
