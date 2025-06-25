package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	Router *gin.Engine
	Port   string
}

// NewHTTPServer 创建新的HTTP服务器
func NewHTTPServer(port string) *HTTPServer {
	router := gin.Default()
	return &HTTPServer{
		Router: router,
		Port:   port,
	}
}

// Start 启动HTTP服务器
func (hs *HTTPServer) Start() error {
	fmt.Printf("协调器启动，监听端口 %s\n", hs.Port)
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
