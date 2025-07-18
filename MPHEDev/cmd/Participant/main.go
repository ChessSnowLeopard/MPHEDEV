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

// KeyGenProgressPush 密钥生成进度结构体
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
	coordinatorIP      string
	coordinatorIPChan  = make(chan string)
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

// ParticipantSelfStatusResponse 参与方自身状态响应
type ParticipantSelfStatusResponse struct {
	ID           int            `json:"id"`
	IP           string         `json:"ip"`
	Port         int            `json:"port"`
	Status       string         `json:"status"`
	DataSplit    string         `json:"data_split"`
	Participants map[int]string `json:"participants"`
}

// OnlineStatusParticipant 在线参与方信息
type OnlineStatusParticipant struct {
	ID            int    `json:"id"`
	URL           string `json:"url"`
	LastHeartbeat string `json:"last_heartbeat"`
	Status        string `json:"status"`
}

// ParticipantOnlineStatusResponse 参与方在线状态响应
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

func main() {
	// 创建参与方实例
	participant := services.NewParticipant()

	// 添加启动日志到OutputBuffer
	participant.LogInfo("参与方启动中...")

	// 获取本机IP并显示
	localIP, err := utils.GetLocalIP()
	if err != nil {
		participant.LogError(fmt.Sprintf("获取本机IP失败: %v", err))
		panic(err)
	}
	participant.LogInfo(fmt.Sprintf("本机IP: %s", localIP))

	// 获取协调器IP
	coordinatorIP := getUserInput("请输入协调器IP地址: ")
	if coordinatorIP == "" {
		participant.LogError("协调器IP地址不能为空")
		panic("协调器IP地址不能为空")
	}

	// 设置协调器URL
	coordinatorURL := fmt.Sprintf("http://%s:8080", coordinatorIP)

	// 1. 注册并获取参数
	setKeyGenProgress("register", "started", "注册参与方")

	// 重试机制：等待协调器后端服务器启动
	maxRetries := 10
	participant.LogInfo(fmt.Sprintf("尝试连接到协调器: %s", coordinatorURL))
	participant.LogInfo(fmt.Sprintf("参与方IP: %s", localIP))

	for i := 0; i < maxRetries; i++ {
		if err := participant.Register(coordinatorURL); err != nil {
			participant.LogError(fmt.Sprintf("注册失败 (尝试 %d/%d): %v", i+1, maxRetries, err))
			participant.LogWarning(fmt.Sprintf("网络诊断: 参与方(%s) -> 协调器(%s)", localIP, coordinatorIP))
			if i < maxRetries-1 {
				participant.LogInfo("等待3秒后重试...")
				time.Sleep(3 * time.Second)
				continue
			}
			setKeyGenProgress("register", "failed", err.Error())
			panic(err)
		}
		break
	}
	setKeyGenProgress("register", "success", "注册成功")
	participant.LogInfo("注册成功")

	// HTTP服务器已经在registry_service.go中启动
	participant.LogInfo(fmt.Sprintf("HTTP服务器已在端口 %d 启动 (参与方ID: %d)", participant.Port, participant.ID))

	// 设置参与方ID到客户端
	participant.CoordinatorClient.SetParticipantID(participant.ID)

	// 捕获Ctrl+C信号，自动注销
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		participant.LogInfo("检测到退出信号，正在注销...")
		if err := participant.Unregister(); err != nil {
			participant.LogError(fmt.Sprintf("注销失败: %v", err))
		} else {
			participant.LogInfo("注销成功")
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
	participant.LogInfo("开始生成伽罗瓦密钥份额...")
	galoisShares, err := keyGen.GenerateGaloisKeyShares()
	if err != nil {
		panic(err)
	}

	participant.LogInfo(fmt.Sprintf("开始上传伽罗瓦密钥份额 (共 %d 个)...", len(galoisShares)))
	shareCount := 0
	for galEl, share := range galoisShares {
		shareCount++
		participant.LogInfo(fmt.Sprintf("正在上传第 %d/%d 个伽罗瓦密钥份额 (GalEl: %d)...", shareCount, len(galoisShares), galEl))

		shareB64, err := keyGen.EncodeGaloisKeyShare(share)
		if err != nil {
			panic(err)
		}
		if err := participant.CoordinatorClient.UploadGaloisKeyShare(galEl, shareB64); err != nil {
			panic(err)
		}
		participant.LogInfo(fmt.Sprintf("第 %d/%d 个伽罗瓦密钥份额上传完成", shareCount, len(galoisShares)))
	}
	participant.LogInfo("所有伽罗瓦密钥份额上传完成")

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
	participant.LogInfo("开始等待所有密钥生成完成...")
	for {
		status, err := participant.CoordinatorClient.PollStatus()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		if status.GlobalPKReady && status.SkAggReady && status.RlkReady &&
			status.CompletedGaloisKeys == status.TotalGaloisKeys {
			participant.LogInfo("所有密钥生成完成！")
			break
		}
		time.Sleep(2 * time.Second)
	}

	// 11. 获取聚合后的密钥
	participant.LogInfo("开始获取聚合后的密钥...")
	keys, err := participant.CoordinatorClient.GetAggregatedKeys()
	if err != nil {
		participant.LogError(fmt.Sprintf("获取聚合密钥失败: %v", err))
		panic(err)
	}

	// 解码并设置公钥
	participant.LogInfo("开始解码并设置公钥...")
	pubKeyBytes, err := utils.DecodeFromBase64(keys.PubKey)
	if err != nil {
		panic(err)
	}
	var pubKey rlwe.PublicKey
	if err := utils.DecodeShare(pubKeyBytes, &pubKey); err != nil {
		panic(err)
	}
	participant.KeyManager.SetPublicKey(&pubKey)
	participant.LogInfo("公钥设置完成")

	// 解码并设置重线性化密钥
	participant.LogInfo("开始解码并设置重线性化密钥...")
	rlkBytes, err := utils.DecodeFromBase64(keys.RelineKey)
	if err != nil {
		panic(err)
	}
	var rlk rlwe.RelinearizationKey
	if err := utils.DecodeShare(rlkBytes, &rlk); err != nil {
		panic(err)
	}
	participant.KeyManager.SetRelinearizationKey(&rlk)
	participant.LogInfo("重线性化密钥设置完成")

	// 解码并设置伽罗瓦密钥
	participant.LogInfo(fmt.Sprintf("开始解码并设置伽罗瓦密钥 (共 %d 个)...", len(keys.GaloisKeys)))
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
	participant.LogInfo("所有伽罗瓦密钥设置完成")

	// 解码并设置协同私钥（仅测试模式）
	if keys.SecretKey != "" {
		participant.LogInfo("开始解码并设置协同私钥（测试模式）...")
		secretKeyBytes, err := utils.DecodeFromBase64(keys.SecretKey)
		if err != nil {
			panic(err)
		}
		var secretKey rlwe.SecretKey
		if err := utils.DecodeShare(secretKeyBytes, &secretKey); err != nil {
			panic(err)
		}
		participant.KeyManager.SetAggregatedSecretKey(&secretKey)
		participant.LogInfo("✓ 协同私钥设置完成（测试模式）")
	} else {
		participant.LogWarning("⚠️  没有协同私钥，跳过私钥设置")
		participant.KeyManager.SetAggregatedSecretKey(nil)
	}

	// 初始化评估器和编码器（在密钥设置完成后）
	participant.LogInfo("开始初始化评估器和编码器...")
	evk := rlwe.NewMemEvaluationKeySet(participant.KeyManager.GetRelinearizationKey(), participant.KeyManager.GetGaloisKeys()...)
	evaluator := ckks.NewEvaluator(ckksParams, evk)
	encoder := ckks.NewEncoder(ckksParams)
	participant.KeyManager.SetEvaluator(evaluator)
	participant.KeyManager.SetEncoder(encoder)
	participant.LogInfo("评估器和编码器初始化完成")

	// 12. 更新数据打包服务，启用加密功能
	participant.LogInfo("开始更新数据打包服务，启用加密功能...")
	if err := participant.UpdateDataPackingServiceWithKeys(); err != nil {
		participant.LogError(fmt.Sprintf("更新数据打包服务失败: %v", err))
		panic(err)
	}

	// 13. 获取在线成员列表
	participant.LogInfo(fmt.Sprintf("参与方 %d 收集密钥并解码设置，启动成功，开始检查在线状态...", participant.ID))
	if err := participant.CheckOnlineStatusBeforeOperation(); err != nil {
		participant.LogError(fmt.Sprintf("在线状态检查失败: %v", err))
		panic(err)
	}

	if err := participant.UpdateOnlineParticipants(); err != nil {
		panic(err)
	}

	// 14. 角色判定
	participant.LogInfo(fmt.Sprintf("参与方 %d 开始角色判定...", participant.ID))
	if err := participant.DetermineRole(); err != nil {
		participant.LogError(fmt.Sprintf("角色判定失败: %v", err))
		panic(err)
	}

	// 15. 载入数据集
	if err := participant.LoadDataset(); err != nil {
		panic(err)
	}

	// 16. 根据角色处理数据集重排打包加密
	participant.LogInfo(fmt.Sprintf("参与方 %d 开始处理数据集重排打包加密...", participant.ID))
	if err := participant.ProcessDatasetWithPacking(); err != nil {
		participant.LogError(fmt.Sprintf("数据集处理失败: %v", err))
		panic(err)
	}

	// 17. 运行主循环
	participant.LogInfo("初始化完成，开始运行主循环...")
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
