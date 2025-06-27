package services

import (
	"MPHEDev/cmd/Coordinator/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// KeysResponse 聚合密钥响应结构体
type KeysResponse struct {
	PubKey     string            `json:"pub_key"`
	RelineKey  string            `json:"reline_key"`
	GaloisKeys map[string]string `json:"galois_keys"`
}

// ==================== HTTP处理器方法 ====================

// registerHandler 注册参与方处理器
func (c *Coordinator) registerHandler(ctx *gin.Context) {
	id := c.RegisterParticipant()
	ctx.JSON(http.StatusOK, gin.H{"participant_id": id})
}

// getCKKSParamsHandler 获取CKKS参数处理器
func (c *Coordinator) getCKKSParamsHandler(ctx *gin.Context) {
	params, crp, galEls, galoisCRPs, rlkCRP, refreshCRS, dataSplitType := c.GetParams()
	ctx.JSON(http.StatusOK, gin.H{
		"params":          params,
		"crp":             crp,
		"gal_els":         galEls,
		"galois_crps":     galoisCRPs,
		"rlk_crp":         rlkCRP,
		"refresh_crs":     refreshCRS,
		"data_split_type": dataSplitType,
	})
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
