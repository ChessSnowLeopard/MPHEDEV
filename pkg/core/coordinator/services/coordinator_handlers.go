package services

import (
	"MPHEDev/pkg/core/coordinator/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// KeysResponse 聚合密钥响应结构体
type KeysResponse struct {
	PubKey     string            `json:"pub_key"`
	RelineKey  string            `json:"reline_key"`
	GaloisKeys map[string]string `json:"galois_keys"`
}

// CoordinatorStartResponse is the response structure for /api/coordinator/init
// 响应体
//
//	type CoordinatorStartResponse struct {
//	    Success             bool   `json:"success"`
//	    Message             string `json:"message"`
//	    CoordinatorID       string `json:"coordinator_id"`
//	    ExpectedParticipants int   `json:"expected_participants"`
//	    DataSplitType       string `json:"data_split_type"`
//	    Status              string `json:"status"`
//	    CoordinatorIP       string `json:"coordinator_ip"`
//	    CoordinatorPort     int    `json:"coordinator_port"`
//	    StartTime           string `json:"start_time"`
//	}
type CoordinatorStartResponse struct {
	Success              bool   `json:"success"`
	Message              string `json:"message"`
	CoordinatorID        string `json:"coordinator_id"`
	ExpectedParticipants int    `json:"expected_participants"`
	DataSplitType        string `json:"data_split_type"`
	Status               string `json:"status"`
	CoordinatorIP        string `json:"coordinator_ip"`
	CoordinatorPort      int    `json:"coordinator_port"`
	StartTime            string `json:"start_time"`
}

// ==================== HTTP处理器方法 ====================

// registerHandler 注册参与方处理器
func (c *Coordinator) registerHandler(ctx *gin.Context) {
	var req struct {
		ShardID string `json:"shard_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.ShardID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request, shard_id required"})
		return
	}
	id := c.RegisterParticipant(req.ShardID)
	ctx.JSON(http.StatusOK, gin.H{"participant_id": id})
}

// unregisterHandler 注销参与方处理器
func (c *Coordinator) unregisterHandler(ctx *gin.Context) {
	var req struct {
		ShardID string `json:"shard_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.ShardID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request, shard_id required"})
		return
	}
	c.UnregisterParticipant(req.ShardID)
	ctx.JSON(http.StatusOK, gin.H{"status": "unregistered"})
}

// getCKKSParamsHandler 获取CKKS参数处理器
func (c *Coordinator) getCKKSParamsHandler(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("参数接口panic: %v\n", r)
		}
	}()
	paramsLiteralB64, galEls, commonCRSSeed, dataSplitType := c.GetParams()
	testObj := gin.H{
		"params_literal":  paramsLiteralB64, // 现在是base64编码的字符串
		"gal_els":         galEls,
		"common_crs_seed": commonCRSSeed, // 统一的CRS种子
		"data_split_type": dataSplitType,
	}
	b, err := json.Marshal(testObj)
	if err != nil {
		fmt.Printf("序列化失败: %v\n", err)
	} else {
		fmt.Printf("参数json大小: %d 字节\n", len(b))
	}
	ctx.JSON(http.StatusOK, testObj)
}

// postPublicKeyHandler 提交公钥份额处理器
func (c *Coordinator) postPublicKeyHandler(ctx *gin.Context) {
	var req utils.PublicKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddPublicKeyShare(req.ParticipantID, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "public key share added"})
}

// postSecretKeyHandler 提交私钥处理器
func (c *Coordinator) postSecretKeyHandler(ctx *gin.Context) {
	var req utils.SecretKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddSecretKey(req.ParticipantID, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "secret key added"})
}

// postGaloisKeyHandler 提交伽罗瓦密钥份额处理器
func (c *Coordinator) postGaloisKeyHandler(ctx *gin.Context) {
	var req utils.GaloisKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddGaloisKeyShare(req.ParticipantID, req.GalEl, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "galois key share added"})
}

// postRelinearizationKeyHandler 提交重线性化密钥份额处理器
func (c *Coordinator) postRelinearizationKeyHandler(ctx *gin.Context) {
	var req utils.RelinearizationKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddRelinearizationKeyShare(req.ParticipantID, req.Round, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "relinearization key share added"})
}

// getRelinearizationKeyRound1AggregatedHandler 获取第一轮重线性化密钥聚合结果处理器
func (c *Coordinator) getRelinearizationKeyRound1AggregatedHandler(ctx *gin.Context) {
	share, err := c.GetRelinearizationKeyRound1Aggregated()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"share": share})
}

// getParticipantsHandler 获取参与方列表处理器
func (c *Coordinator) getParticipantsHandler(ctx *gin.Context) {
	participants := c.GetParticipants()
	ctx.JSON(http.StatusOK, participants)
}

// getSetupStatusHandler 获取设置状态处理器
func (c *Coordinator) getSetupStatusHandler(ctx *gin.Context) {
	status := c.GetStatus()
	ctx.JSON(http.StatusOK, status)
}

// reportURLHandler 报告URL处理器
func (c *Coordinator) reportURLHandler(ctx *gin.Context) {
	var req utils.PeerInfo
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := c.AddParticipantURL(req.ID, req.URL); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "url_reported"})
}

// getParticipantsListHandler 获取参与方URL列表处理器
func (c *Coordinator) getParticipantsListHandler(ctx *gin.Context) {
	participants := c.GetAllParticipantURLs()
	ctx.JSON(http.StatusOK, participants)
}

// heartbeatHandler 心跳处理器
func (c *Coordinator) heartbeatHandler(ctx *gin.Context) {
	var req struct {
		ParticipantID int `json:"participant_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := c.UpdateHeartbeat(req.ParticipantID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "heartbeat_updated"})
}

// getOnlineParticipantsHandler 获取在线参与方处理器
func (c *Coordinator) getOnlineParticipantsHandler(ctx *gin.Context) {
	onlineParticipants := c.GetOnlineParticipants()
	ctx.JSON(http.StatusOK, onlineParticipants)
}

// getOnlineStatusHandler 获取在线状态处理器
func (c *Coordinator) getOnlineStatusHandler(ctx *gin.Context) {
	status := c.GetOnlineStatus()
	ctx.JSON(http.StatusOK, status)
}

// getDetailedStatusHandler 获取详细状态处理器（类似NetworkDemo的状态页面）
func (c *Coordinator) getDetailedStatusHandler(ctx *gin.Context) {
	// 获取在线状态
	onlineStatus := c.GetOnlineStatus()

	// 获取所有参与方信息
	participants := c.GetParticipants()

	// 获取在线参与方列表
	onlineParticipants := c.GetOnlineParticipants()

	// 构造详细状态响应
	detailedStatus := gin.H{
		"coordinator_ip":           c.GetLocalIP(),
		"port":                     8080,
		"total_participants":       len(participants),
		"online_participants":      len(onlineParticipants),
		"online_percentage":        onlineStatus["online_percentage"],
		"min_participants":         onlineStatus["min_participants"],
		"can_proceed":              onlineStatus["can_proceed"],
		"online_timeout":           onlineStatus["online_timeout"],
		"heartbeat_interval":       onlineStatus["heartbeat_interval"],
		"participants":             participants,
		"online_participants_list": onlineParticipants,
	}

	ctx.JSON(http.StatusOK, detailedStatus)
}

// testAllKeysHandler 测试所有密钥处理器
func (c *Coordinator) testAllKeysHandler(ctx *gin.Context) {
	if err := c.TestAllKeys(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "all keys tested"})
}

// testPublicKeyHandler 测试公钥处理器
func (c *Coordinator) testPublicKeyHandler(ctx *gin.Context) {
	if err := c.TestPublicKeyOnly(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "public key tested"})
}

// testRelinearizationKeyHandler 测试重线性化密钥处理器
func (c *Coordinator) testRelinearizationKeyHandler(ctx *gin.Context) {
	if err := c.TestRelinearizationKeyOnly(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "relinearization key tested"})
}

// testGaloisKeysHandler 测试伽罗瓦密钥处理器
func (c *Coordinator) testGaloisKeysHandler(ctx *gin.Context) {
	if err := c.TestGaloisKeysOnly(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "galois keys tested"})
}

// getRelinearizationKeyStatusHandler 获取重线性化密钥状态处理器
func (c *Coordinator) getRelinearizationKeyStatusHandler(ctx *gin.Context) {
	status := c.GetStatus()
	ctx.JSON(http.StatusOK, status)
}

// getAggregatedKeysHandler 获取聚合密钥处理器
func (c *Coordinator) getAggregatedKeysHandler(ctx *gin.Context) {
	// 检查所有密钥是否都已准备就绪
	status := c.GetStatus()
	if !status["global_pk_ready"].(bool) || !status["rlk_ready"].(bool) {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "密钥尚未完全聚合完成"})
		return
	}

	// 获取聚合后的密钥
	pubKey := c.KeyManager.GetGlobalPK()
	relineKey := c.KeyManager.GetRelinearizationKey()
	galoisKeys := c.KeyManager.GetGaloisKeys()
	galEls := c.ParameterManager.GetGalEls() // 获取伽罗瓦元素顺序

	// 序列化公钥
	pubKeyBytes, err := utils.EncodeShare(pubKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "公钥序列化失败"})
		return
	}
	pubKeyB64 := utils.EncodeToBase64(pubKeyBytes)

	// 序列化重线性化密钥
	relineKeyBytes, err := utils.EncodeShare(relineKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "重线性化密钥序列化失败"})
		return
	}
	relineKeyB64 := utils.EncodeToBase64(relineKeyBytes)

	// 序列化伽罗瓦密钥
	galoisKeysMap := make(map[string]string)
	for i, galEl := range galEls {
		if i < len(galoisKeys) && galoisKeys[i] != nil {
			gkBytes, err := utils.EncodeShare(galoisKeys[i])
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "伽罗瓦密钥序列化失败"})
				return
			}
			galoisKeysMap[strconv.FormatUint(galEl, 10)] = utils.EncodeToBase64(gkBytes)
		}
	}

	resp := KeysResponse{
		PubKey:     pubKeyB64,
		RelineKey:  relineKeyB64,
		GaloisKeys: galoisKeysMap,
	}
	ctx.JSON(http.StatusOK, resp)
}

// ==================== 密钥分发方法 ====================

// DistributeKeysToParticipants 分发密钥给参与方
func (c *Coordinator) DistributeKeysToParticipants() error {
	onlineParticipants := c.ParticipantManager.GetOnlineParticipants()
	if len(onlineParticipants) < c.ParticipantManager.GetMinParticipants() {
		return fmt.Errorf("在线参与方数量不足: %d/%d", len(onlineParticipants), c.ParticipantManager.GetMinParticipants())
	}

	// 在分发密钥前先进行最终测试
	fmt.Println("开始最终密钥测试...")
	if err := c.TestAllKeys(); err != nil {
		return fmt.Errorf("密钥测试失败，无法分发: %v", err)
	}
	fmt.Println("所有密钥测试通过，开始分发...")

	// 构造密钥数据
	keysData := map[string]interface{}{
		"params":      c.ParameterManager.GetParamsLiteral(),
		"pub_key":     c.KeyManager.GetGlobalPK(),
		"reline_key":  c.KeyManager.GetRelinearizationKey(),
		"galois_keys": c.KeyManager.GetGaloisKeys(),
	}

	// 向所有在线参与方分发密钥
	for _, participant := range onlineParticipants {
		if err := c.postJSON(participant.URL+"/keys/receive", keysData); err != nil {
			fmt.Printf("向参与方 %d 分发密钥失败: %v\n", participant.ID, err)
		} else {
			fmt.Printf("向参与方 %d 分发密钥成功\n", participant.ID)
		}
	}

	return nil
}

// postJSON 发送JSON请求
func (c *Coordinator) postJSON(url string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP请求失败: %d", resp.StatusCode)
	}

	return nil
}

// ==================== 协调器初始化接口和中间件 ====================

type InitRequest struct {
	NumParticipants int    `json:"num_participants"`
	DataSplitType   string `json:"data_split_type"` // "horizontal" or "vertical"
}

var (
	globalCoordinator *Coordinator
)

// InitHandler 初始化协调器
func InitHandler(ctx *gin.Context) {
	var req InitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.NumParticipants <= 0 {
		ctx.JSON(400, gin.H{"error": "invalid num_participants"})
		return
	}
	dataSplitType := req.DataSplitType
	if v, ok := ctx.Get("data_split_type"); ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			dataSplitType = s
		}
	}
	coordinator, err := NewCoordinator(req.NumParticipants, dataSplitType)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	globalCoordinator = coordinator
	go globalCoordinator.Start() // 启动后台服务

	coordinatorID := uuid.New().String()
	startTime := time.Now().Format(time.RFC3339)
	ip := globalCoordinator.GetLocalIP()
	port := 8080 // 默认端口
	resp := CoordinatorStartResponse{
		Success:              true,
		Message:              "Coordinator initialized successfully",
		CoordinatorID:        coordinatorID,
		ExpectedParticipants: req.NumParticipants,
		DataSplitType:        dataSplitType,
		Status:               "running",
		CoordinatorIP:        ip,
		CoordinatorPort:      port,
		StartTime:            startTime,
	}
	ctx.JSON(200, resp)
}

// RequireCoordinator Gin 中间件，校验 globalCoordinator 是否已初始化
func RequireCoordinator() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if globalCoordinator == nil {
			ctx.JSON(400, gin.H{"error": "Coordinator not initialized"})
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func PostPublicKeyHandler(ctx *gin.Context) {
	globalCoordinator.postPublicKeyHandler(ctx)
}

func PostSecretKeyHandler(ctx *gin.Context) {
	globalCoordinator.postSecretKeyHandler(ctx)
}

func GetSetupStatusHandler(ctx *gin.Context) {
	globalCoordinator.getSetupStatusHandler(ctx)
}
