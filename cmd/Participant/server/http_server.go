package server

import (
	"fmt"
	"net/http"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	Server *http.Server
	Port   int
}

// NewHTTPServer 创建新的HTTP服务器
func NewHTTPServer(port int, handlers map[string]http.HandlerFunc) *HTTPServer {
	mux := http.NewServeMux()

	// 添加路由处理器
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &HTTPServer{
		Server: server,
		Port:   port,
	}
}

// Start 启动HTTP服务器
func (hs *HTTPServer) Start(participantID int) error {
	go func() {
		if err := hs.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("参与方 %d 服务器启动失败: %v\n", participantID, err)
		}
	}()

	fmt.Printf("参与方 %d P2P服务器启动在端口 %d\n", participantID, hs.Port)
	return nil
}

// Stop 停止HTTP服务器
func (hs *HTTPServer) Stop() error {
	return hs.Server.Close()
}
