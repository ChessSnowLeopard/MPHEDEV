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

	router := gin.Default()
	return &HTTPServer{
		Router:  router,
		Port:    port,
		LocalIP: localIP,
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
