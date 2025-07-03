package services

import (
	"MPHEDev/pkg/core/coordinator/keys"
	"MPHEDev/pkg/core/coordinator/parameters"
	"MPHEDev/pkg/core/coordinator/participants"
	"MPHEDev/pkg/core/coordinator/server"
	"MPHEDev/pkg/core/coordinator/utils"
	"fmt"

	"github.com/gin-gonic/gin"
)

// Coordinator 重构后的协调器主结构体
type Coordinator struct {
	// 参与者管理
	ParticipantManager *participants.Manager

	// 参数管理
	ParameterManager *parameters.Manager

	// 密钥管理
	KeyManager *keys.Manager

	// 密钥聚合
	KeyAggregator *keys.Aggregator

	// 密钥测试
	KeyTester *keys.Tester

	// HTTP服务器
	HTTPServer *server.HTTPServer

	// 状态管理
	expectedN int
}

// NewCoordinator 创建新的协调器实例
func NewCoordinator(expectedN int, dataSplitType string) (*Coordinator, error) {
	// 创建参数管理器
	paramManager, err := parameters.NewManager(dataSplitType)
	if err != nil {
		return nil, fmt.Errorf("创建参数管理器失败: %v", err)
	}

	// 创建参与者管理器
	participantManager := participants.NewManager(expectedN)

	// 创建密钥管理器
	keyManager := keys.NewManager(paramManager.GetCKKSParams(), expectedN)

	// 创建密钥聚合器
	keyAggregator := keys.NewAggregator(keyManager)

	// 创建密钥测试器
	keyTester := keys.NewTester(keyManager)

	// 创建HTTP服务器
	httpServer := server.NewHTTPServer("8080")

	coordinator := &Coordinator{
		ParticipantManager: participantManager,
		ParameterManager:   paramManager,
		KeyManager:         keyManager,
		KeyAggregator:      keyAggregator,
		KeyTester:          keyTester,
		HTTPServer:         httpServer,
		expectedN:          expectedN,
	}

	// 设置路由
	coordinator.setupRoutes()

	return coordinator, nil
}

// setupRoutes 设置HTTP路由
func (c *Coordinator) setupRoutes() {
	router := c.HTTPServer.GetRouter()

	// 注册路由处理器
	router.POST("/register", c.registerHandler)
	router.GET("/params/ckks", c.getCKKSParamsHandler)
	router.POST("/keys/public", c.postPublicKeyHandler)
	router.POST("/keys/secret", c.postSecretKeyHandler)
	router.POST("/keys/galois", c.postGaloisKeyHandler)
	router.POST("/keys/relin", c.postRelinearizationKeyHandler)
	router.GET("/keys/relin/round1", c.getRelinearizationKeyRound1AggregatedHandler)
	router.GET("/keys/aggregated", c.getAggregatedKeysHandler)
	router.GET("/participants", c.getParticipantsHandler)
	router.GET("/setup/status", c.getSetupStatusHandler)

	// P2P相关路由
	router.POST("/participants/url", c.reportURLHandler)
	router.GET("/participants/list", c.getParticipantsListHandler)

	// 在线状态管理路由
	router.POST("/heartbeat", c.heartbeatHandler)
	router.GET("/participants/online", c.getOnlineParticipantsHandler)
	router.GET("/status/online", c.getOnlineStatusHandler)
	router.GET("/status", c.getDetailedStatusHandler)

	// 测试相关路由
	router.POST("/test/all", c.testAllKeysHandler)
	router.POST("/test/public", c.testPublicKeyHandler)
	router.POST("/test/relin", c.testRelinearizationKeyHandler)
	router.POST("/test/galois", c.testGaloisKeysHandler)

	// 重线性化密钥状态查询路由
	router.GET("/keys/relin/status", c.getRelinearizationKeyStatusHandler)

	router.POST("/unregister", c.unregisterHandler)
}

// Start 启动协调器
func (c *Coordinator) Start() error {
	// 启动心跳清理协程
	c.ParticipantManager.StartHeartbeatCleanup()

	// 启动HTTP服务器
	return c.HTTPServer.Start()
}

// ==================== 参与者管理方法 ====================

// RegisterParticipant 注册新参与方
func (c *Coordinator) RegisterParticipant(shardID string) int {
	return c.ParticipantManager.RegisterParticipant(shardID)
}

// UnregisterParticipant 注销参与方
func (c *Coordinator) UnregisterParticipant(shardID string) {
	c.ParticipantManager.UnregisterParticipant(shardID)
}

// AddParticipantURL 添加参与方URL
func (c *Coordinator) AddParticipantURL(participantID int, url string) error {
	return c.ParticipantManager.AddParticipantURL(participantID, url)
}

// GetParticipants 获取所有参与方信息
func (c *Coordinator) GetParticipants() []*utils.ParticipantInfo {
	return c.ParticipantManager.GetParticipants()
}

// GetAllParticipantURLs 获取所有参与方URL列表
func (c *Coordinator) GetAllParticipantURLs() []utils.PeerInfo {
	return c.ParticipantManager.GetAllParticipantURLs()
}

// UpdateHeartbeat 更新参与方心跳
func (c *Coordinator) UpdateHeartbeat(participantID int) error {
	return c.ParticipantManager.UpdateHeartbeat(participantID)
}

// GetOnlineParticipants 获取在线参与方列表
func (c *Coordinator) GetOnlineParticipants() []utils.PeerInfo {
	return c.ParticipantManager.GetOnlineParticipants()
}

// GetOnlineStatus 获取在线状态信息
func (c *Coordinator) GetOnlineStatus() gin.H {
	return c.ParticipantManager.GetOnlineStatus()
}

// GetMinParticipants 获取最小参与方数量
func (c *Coordinator) GetMinParticipants() int {
	return c.ParticipantManager.GetMinParticipants()
}

// GetLocalIP 获取本机IP地址
func (c *Coordinator) GetLocalIP() string {
	return c.HTTPServer.GetLocalIP()
}

// ==================== 参数管理方法 ====================

// GetParams 获取参数
func (c *Coordinator) GetParams() (string, []uint64, string, string) {
	return c.ParameterManager.GetParams()
}
