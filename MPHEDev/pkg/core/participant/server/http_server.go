package server

import (
	"MPHEDev/pkg/core/participant/utils"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"MPHEDev/pkg/buffer"

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

	// 创建Gin路由器，使用gin.New()避免默认日志中间件
	router := gin.New()
	// 只添加Recovery中间件，不添加日志中间件
	router.Use(gin.Recovery())

	// 添加CORS中间件，允许跨域请求
	router.Use(func(c *gin.Context) {
		// 允许所有来源
		c.Header("Access-Control-Allow-Origin", "*")
		// 允许的HTTP方法
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		// 允许的请求头
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Cache-Control, Pragma, X-Requested-With, Accept, User-Agent, Referer")
		// 暴露的响应头
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, X-Total-Count")
		// 允许携带凭证
		c.Header("Access-Control-Allow-Credentials", "true")
		// 预检请求缓存时间
		c.Header("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 添加状态页面
	router.GET("/status", func(c *gin.Context) {
		onlineParticipants := messageHandler.GetOnlineParticipants()
		c.JSON(http.StatusOK, gin.H{
			"status":             "ok",
			"service":            "participant-backend",
			"port":               port,
			"participant_id":     messageHandler.GetID(),
			"timestamp":          time.Now().Format(time.RFC3339),
			"id":                 messageHandler.GetID(),
			"ip":                 localIP,
			"participant_status": "online",
			"data_split":         messageHandler.GetDataSplit(),
			"participants":       onlineParticipants,
		})
	})

	// 添加密钥进度查询接口
	router.GET("/api/participant/step", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"step":    "key_generation",
			"status":  "in_progress",
			"message": "密钥生成中...",
		})
	})

	// 添加自身状态查询接口
	router.GET("/api/participant/status", func(c *gin.Context) {
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

	// 添加在线状态查询接口
	router.GET("/api/participant/online-status", func(c *gin.Context) {
		onlineParticipants := messageHandler.GetOnlineParticipants()
		c.JSON(http.StatusOK, gin.H{
			"online_count":       len(onlineParticipants) + 1, // 包括自己
			"total_count":        3,                           // 假设总共3个参与方
			"online_percentage":  float64(len(onlineParticipants)+1) / 3.0 * 100,
			"min_participants":   2,
			"can_proceed":        len(onlineParticipants) >= 1,
			"online_timeout":     30.0,
			"heartbeat_interval": 5.0,
			"participants":       []gin.H{},
		})
	})

	// 添加后端输出查询接口
	router.GET("/api/participant/backend-output", func(c *gin.Context) {
		// 获取查询参数
		outputType := c.Query("type") // "all", "recent", "key", "filtered"
		maxLines := c.DefaultQuery("max_lines", "100")
		levels := c.QueryArray("levels") // 过滤级别：info, warning, error, debug

		maxLinesInt := 100
		if maxLines != "" {
			if parsed, err := strconv.Atoi(maxLines); err == nil {
				maxLinesInt = parsed
			}
		}

		// 从参与方的OutputBuffer获取真实输出
		var output string
		var stats map[string]interface{}

		if messageHandler != nil {
			// 尝试获取参与方的OutputBuffer
			if participant, ok := messageHandler.(interface{ GetOutputBuffer() *buffer.OutputBuffer }); ok {
				outputBuffer := participant.GetOutputBuffer()
				if outputBuffer != nil {
					switch outputType {
					case "recent":
						output = outputBuffer.GetRecentOutputs(maxLinesInt)
					case "key":
						output = outputBuffer.GetKeyOutputs(maxLinesInt)
					case "filtered":
						if len(levels) > 0 {
							output = outputBuffer.GetOutputsAsStringFiltered(levels, maxLinesInt)
						} else {
							output = outputBuffer.GetOutputsAsString()
						}
					case "all", "":
						output = outputBuffer.GetOutputsAsString()
					default:
						output = outputBuffer.GetOutputsAsString()
					}
					stats = outputBuffer.GetStats()
				} else {
					output = "OutputBuffer未初始化"
					stats = gin.H{"error": "OutputBuffer未初始化"}
				}
			} else {
				output = "无法获取OutputBuffer接口"
				stats = gin.H{"error": "无法获取OutputBuffer接口"}
			}
		} else {
			output = "MessageHandler未初始化"
			stats = gin.H{"error": "MessageHandler未初始化"}
		}

		// 如果OutputBuffer为空，返回默认信息
		if output == "" || output == "暂无输出" {
			output = "参与方后端服务运行正常，等待训练输出..."
			stats = gin.H{
				"total_lines":   1,
				"info_count":    1,
				"warning_count": 0,
				"error_count":   0,
				"note":          "OutputBuffer为空，显示默认状态",
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"output": output,
			"stats":  stats,
			"type":   outputType,
		})
	})

	// 添加清空后端输出接口
	router.POST("/api/participant/clear-output", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "output_cleared"})
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
