package main

import (
	"MPHEDev/pkg/core/participant/services"
	"MPHEDev/pkg/core/participant/utils"
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// getUserInput 获取用户输入
func getUserInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// ParticipantIPPush WebSocket 消息结构体
// type ParticipantIPPush struct {
//     Type string `json:"type"`
//     IP   string `json:"ip"`
//     Port int    `json:"port"`
// }

// KeyGenProgressPush 结构体
// 用于 /api/participant/step 接口
type KeyGenProgressPush struct {
	Type      string `json:"type"`
	Step      string `json:"step"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

var (
	keyGenProgress     = KeyGenProgressPush{Type: "keygen_progress"}
	keyGenProgressLock sync.RWMutex
)

func setKeyGenProgress(step, status, message string) {
	keyGenProgressLock.Lock()
	defer keyGenProgressLock.Unlock()
	keyGenProgress.Step = step
	keyGenProgress.Status = status
	keyGenProgress.Message = message
	keyGenProgress.Timestamp = time.Now().Format(time.RFC3339)
}

func getKeyGenProgress() KeyGenProgressPush {
	keyGenProgressLock.RLock()
	defer keyGenProgressLock.RUnlock()
	return keyGenProgress
}

// ParticipantSelfStatusResponse 结构体
//
//	type ParticipantSelfStatusResponse struct {
//	    ID          int               `json:"id"`
//	    IP          string            `json:"ip"`
//	    Port        int               `json:"port"`
//	    Status      string            `json:"status"`
//	    DataSplit   string            `json:"data_split"`
//	    Participants map[int]string   `json:"participants"`
//	}
type ParticipantSelfStatusResponse struct {
	ID           int            `json:"id"`
	IP           string         `json:"ip"`
	Port         int            `json:"port"`
	Status       string         `json:"status"`
	DataSplit    string         `json:"data_split"`
	Participants map[int]string `json:"participants"`
}

// OnlineStatusParticipant 和 ParticipantOnlineStatusResponse 结构体
//
//	type OnlineStatusParticipant struct {
//	    ID            int    `json:"id"`
//	    URL           string `json:"url"`
//	    LastHeartbeat string `json:"last_heartbeat"`
//	    Status        string `json:"status"`
//	}
//
//	type ParticipantOnlineStatusResponse struct {
//	    OnlineCount        int                       `json:"online_count"`
//	    TotalCount         int                       `json:"total_count"`
//	    OnlinePercentage   float64                   `json:"online_percentage"`
//	    MinParticipants    int                       `json:"min_participants"`
//	    CanProceed         bool                      `json:"can_proceed"`
//	    OnlineTimeout      float64                   `json:"online_timeout"`
//	    HeartbeatInterval  float64                   `json:"heartbeat_interval"`
//	    Participants       []OnlineStatusParticipant `json:"participants"`
//	}
type OnlineStatusParticipant struct {
	ID            int    `json:"id"`
	URL           string `json:"url"`
	LastHeartbeat string `json:"last_heartbeat"`
	Status        string `json:"status"`
}
type ParticipantOnlineStatusResponse struct {
	OnlineCount       int                       `json:"online_count"`
	TotalCount        int                       `json:"total_count"`
	OnlinePercentage  float64                   `json:"online_percentage"`
	MinParticipants   int                       `json:"min_participants"`
	CanProceed        bool                      `json:"can_proceed"`
	OnlineTimeout     float64                   `json:"online_timeout"`
	HeartbeatInterval float64                   `json:"heartbeat_interval"`
	Participants      []OnlineStatusParticipant `json:"participants"`
}

func startIPPushServer(ip string, port int, participant *services.Participant) {
	r := gin.Default()
	r.GET("/api/participant/ws", func(c *gin.Context) {
		msg := struct {
			Type string `json:"type"`
			IP   string `json:"ip"`
			Port int    `json:"port"`
		}{
			Type: "ip",
			IP:   ip,
			Port: port,
		}
		c.JSON(200, msg)
	})
	r.POST("/api/participant/ws", func(c *gin.Context) {
		msg := struct {
			Type string `json:"type"`
			IP   string `json:"ip"`
			Port int    `json:"port"`
		}{
			Type: "ip",
			IP:   ip,
			Port: port,
		}
		c.JSON(200, msg)
	})
	// 新增密钥进度查询接口
	r.GET("/api/participant/step", func(c *gin.Context) {
		c.JSON(200, getKeyGenProgress())
	})
	// 新增自身状态查询接口
	r.GET("/api/participant/status", func(c *gin.Context) {
		resp := ParticipantSelfStatusResponse{
			ID:           participant.ID,
			IP:           ip,
			Port:         port,
			Status:       "online",
			DataSplit:    participant.DataSplit,
			Participants: participant.GetOnlineParticipants(),
		}
		c.JSON(200, resp)
	})
	// 新增在线状态查询接口
	r.GET("/api/participant/online-status", func(c *gin.Context) {
		if participant.HeartbeatManager == nil {
			c.JSON(500, gin.H{"error": "HeartbeatManager not initialized"})
			return
		}
		status, err := participant.HeartbeatManager.GetOnlineStatus()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		resp := ParticipantOnlineStatusResponse{
			OnlineCount:       intValue(status["online_count"]),
			TotalCount:        intValue(status["total_count"]),
			OnlinePercentage:  floatValue(status["online_percentage"]),
			MinParticipants:   intValue(status["min_participants"]),
			CanProceed:        boolValue(status["can_proceed"]),
			OnlineTimeout:     floatValue(status["online_timeout"]),
			HeartbeatInterval: floatValue(status["heartbeat_interval"]),
		}
		peers := participant.PeerManager.GetPeers()
		var participants []OnlineStatusParticipant
		for id, url := range peers {
			participants = append(participants, OnlineStatusParticipant{
				ID:     id,
				URL:    url,
				Status: "online",
			})
		}
		resp.Participants = participants
		c.JSON(200, resp)
	})
	addr := ":8061"
	go func() {
		if err := r.Run(addr); err != nil {
			panic(err)
		}
	}()
}

func main() {
	fmt.Println("参与方启动中...")

	// 创建参与方实例
	participant := services.NewParticipant()

	// 获取本机IP并显示
	localIP, err := utils.GetLocalIP()
	if err != nil {
		fmt.Printf("获取本机IP失败: %v\n", err)
		panic(err)
	}
	fmt.Printf("本机IP: %s\n", localIP)

	// 启动 WebSocket 服务（8061端口）
	startIPPushServer(localIP, 8061, participant)

	// 获取协调器IP
	coordinatorIP := getUserInput("请输入协调器IP地址: ")
	if coordinatorIP == "" {
		fmt.Println("协调器IP地址不能为空")
		panic("协调器IP地址不能为空")
	}

	// 设置协调器URL
	coordinatorURL := fmt.Sprintf("http://%s:8080", coordinatorIP)

	// 1. 注册并获取参数
	setKeyGenProgress("register", "started", "注册参与方")
	if err := participant.Register(coordinatorURL); err != nil {
		setKeyGenProgress("register", "failed", err.Error())
		panic(err)
	}
	setKeyGenProgress("register", "success", "注册成功")

	// 设置参与方ID到客户端
	participant.CoordinatorClient.SetParticipantID(participant.ID)

	// 捕获Ctrl+C信号，自动注销
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n检测到退出信号，正在注销...")
		if err := participant.Unregister(); err != nil {
			fmt.Printf("注销失败: %v\n", err)
		} else {
			fmt.Println("注销成功")
		}
		os.Exit(0)
	}()

	// 2. 获取CKKS参数、CRP和伽罗瓦密钥相关参数
	params, err := participant.CoordinatorClient.GetParams()
	if err != nil {
		panic(err)
	}

	// 将 ParamsResponse 转换为 ckks.Parameters
	ckksParams, err := ckks.NewParametersFromLiteral(params.Params)
	if err != nil {
		panic(err)
	}

	participant.KeyManager.SetParams(ckksParams)
	participant.KeyManager.TotalGaloisKeys = len(params.GalEls)

	// 设置刷新服务的参数和CRS
	participant.RefreshService.UpdateParams(ckksParams)

	// 使用统一的CRS种子设置刷新服务
	commonCRSSeedBytes, err := utils.DecodeFromBase64(params.CommonCRSSeed)
	if err != nil {
		panic(err)
	}
	participant.RefreshService.SetCommonCRSSeed(commonCRSSeedBytes)

	// 3. 生成本地私钥和公钥份额

	// 根据统一CRS种子生成所有CRP
	if err := participant.GenerateAllCRPs(params); err != nil {
		panic(err)
	}

	// 解码生成的CRP
	crpBytes, err := utils.DecodeFromBase64(params.Crp)
	if err != nil {
		panic(err)
	}
	var crp multiparty.PublicKeyGenCRP
	if err := utils.DecodeShare(crpBytes, &crp); err != nil {
		panic(err)
	}

	// 解码生成的GaloisCRPs
	galoisCRPs := make(map[uint64]multiparty.GaloisKeyGenCRP)
	for galEl, crpStr := range params.GaloisCRPs {
		crpBytes, err := utils.DecodeFromBase64(crpStr)
		if err != nil {
			panic(err)
		}
		var galoisCRP multiparty.GaloisKeyGenCRP
		if err := utils.DecodeShare(crpBytes, &galoisCRP); err != nil {
			panic(err)
		}
		galoisCRPs[galEl] = galoisCRP
	}

	// 解码生成的RlkCRP
	rlkCRPBytes, err := utils.DecodeFromBase64(params.RlkCRP)
	if err != nil {
		panic(err)
	}
	var rlkCRP multiparty.RelinearizationKeyGenCRP
	if err := utils.DecodeShare(rlkCRPBytes, &rlkCRP); err != nil {
		panic(err)
	}

	keyGen := services.NewKeyGenerator(ckksParams, &crp, params.GalEls, galoisCRPs, &rlkCRP)
	sk, share, err := keyGen.GenerateKeys()
	if err != nil {
		panic(err)
	}
	participant.KeyManager.SetSecretKey(sk)

	// 4. 编码并上传私钥  该方法仅用于测试环境
	skB64, err := keyGen.EncodeSecretKey(sk)
	if err != nil {
		panic(err)
	}
	setKeyGenProgress("upload_secret_key", "started", "上传私钥")
	if err := participant.CoordinatorClient.UploadSecretKey(skB64); err != nil {
		setKeyGenProgress("upload_secret_key", "failed", err.Error())
		panic(err)
	}
	setKeyGenProgress("upload_secret_key", "success", "上传私钥成功")

	// 5. 编码并上传公钥份额
	shareB64, err := keyGen.EncodePublicKeyShare(share)
	if err != nil {
		panic(err)
	}
	setKeyGenProgress("upload_public_key_share", "started", "上传公钥份额")
	if err := participant.CoordinatorClient.UploadPublicKeyShare(shareB64); err != nil {
		setKeyGenProgress("upload_public_key_share", "failed", err.Error())
		panic(err)
	}
	setKeyGenProgress("upload_public_key_share", "success", "上传公钥份额成功")

	// 6. 生成并上传伽罗瓦密钥份额
	galoisShares, err := keyGen.GenerateGaloisKeyShares()
	if err != nil {
		panic(err)
	}

	for galEl, share := range galoisShares {
		shareB64, err := keyGen.EncodeGaloisKeyShare(share)
		if err != nil {
			panic(err)
		}
		if err := participant.CoordinatorClient.UploadGaloisKeyShare(galEl, shareB64); err != nil {
			panic(err)
		}
	}

	// 7. 生成并上传重线性化密钥第一轮份额
	if err := keyGen.GenerateRelinearizationKeyRound1(); err != nil {
		panic(err)
	}
	rlkShare1B64, err := keyGen.EncodeRelinearizationKeyShare(1)
	if err != nil {
		panic(err)
	}
	if err := participant.CoordinatorClient.UploadRelinearizationKeyShare(1, rlkShare1B64); err != nil {
		panic(err)
	}

	// 8. 等待第一轮聚合完成，然后获取聚合结果
	for {
		status, err := participant.CoordinatorClient.PollStatus()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if status.RlkRound1Ready {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// 9. 获取聚合后的第一轮份额，生成第二轮份额
	aggregatedShare1, err := participant.CoordinatorClient.GetRelinearizationKeyRound1Aggregated()
	if err != nil {
		panic(err)
	}
	if err := keyGen.GenerateRelinearizationKeyRound2(aggregatedShare1); err != nil {
		panic(err)
	}
	rlkShare2B64, err := keyGen.EncodeRelinearizationKeyShare(2)
	if err != nil {
		panic(err)
	}
	if err := participant.CoordinatorClient.UploadRelinearizationKeyShare(2, rlkShare2B64); err != nil {
		panic(err)
	}

	// 10. 等待所有密钥生成完成
	for {
		status, err := participant.CoordinatorClient.PollStatus()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if status.GlobalPKReady && status.SkAggReady && status.RlkReady &&
			status.CompletedGaloisKeys == status.TotalGaloisKeys {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// 11. 获取聚合后的密钥
	keys, err := participant.CoordinatorClient.GetAggregatedKeys()
	if err != nil {
		panic(err)
	}

	// 解码并设置公钥
	pubKeyBytes, err := utils.DecodeFromBase64(keys.PubKey)
	if err != nil {
		panic(err)
	}
	var pubKey rlwe.PublicKey
	if err := utils.DecodeShare(pubKeyBytes, &pubKey); err != nil {
		panic(err)
	}
	participant.KeyManager.SetPublicKey(&pubKey)

	// 解码并设置重线性化密钥
	rlkBytes, err := utils.DecodeFromBase64(keys.RelineKey)
	if err != nil {
		panic(err)
	}
	var rlk rlwe.RelinearizationKey
	if err := utils.DecodeShare(rlkBytes, &rlk); err != nil {
		panic(err)
	}
	participant.KeyManager.SetRelinearizationKey(&rlk)

	// 解码并设置伽罗瓦密钥
	galoisKeys := make([]*rlwe.GaloisKey, 0)
	for _, keyStr := range keys.GaloisKeys {
		keyBytes, err := utils.DecodeFromBase64(keyStr)
		if err != nil {
			panic(err)
		}
		var galoisKey rlwe.GaloisKey
		if err := utils.DecodeShare(keyBytes, &galoisKey); err != nil {
			panic(err)
		}
		galoisKeys = append(galoisKeys, &galoisKey)
	}
	participant.KeyManager.SetGaloisKeys(galoisKeys)

	// 12. 获取在线成员列表
	if err := participant.UpdateOnlineParticipants(); err != nil {
		panic(err)
	}

	fmt.Printf("参与方 %d 启动成功\n", participant.ID)

	// 13. 载入数据集
	if err := participant.LoadDataset(); err != nil {
		panic(err)
	}

	// 14. 加密并分发数据集
	if err := participant.EncryptAndDistributeDataset(); err != nil {
		panic(err)
	}

	// 15. 等待数据分发完成
	<-participant.ReadyCh

	// 16. 运行主循环
	participant.RunMainLoop()
}

// 辅助类型转换函数
func intValue(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	}
	return 0
}
func floatValue(v interface{}) float64 {
	if v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	if i, ok := v.(int); ok {
		return float64(i)
	}
	return 0
}
func boolValue(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
