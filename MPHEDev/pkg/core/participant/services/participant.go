package services

import (
	"MPHEDev/pkg/core/participant/coordinator"
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/data_management"
	"MPHEDev/pkg/core/participant/hidden_layer"
	"MPHEDev/pkg/core/participant/input_layer"
	"MPHEDev/pkg/core/participant/interfaces"
	"MPHEDev/pkg/core/participant/network"
	"MPHEDev/pkg/core/participant/output_layer"
	"MPHEDev/pkg/core/participant/server"
	"MPHEDev/pkg/core/participant/sync"
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"fmt"
	"net/http"
	"strings"
	"time"

	"MPHEDev/pkg/buffer"

	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// Participant 参与方
type Participant struct {
	ID int
	// 网络相关
	Client            *types.HTTPClient
	PeerManager       *network.PeerManager
	HTTPServer        *server.HTTPServer
	HeartbeatManager  *network.HeartbeatManager
	MessageProcessor  *MessageProcessor
	RegistryService   *RegistryService
	CoordinatorClient *coordinator.CoordinatorClient

	// 密钥管理
	KeyManager        *crypto.KeyManager
	DecryptionService *crypto.DecryptionService
	RefreshService    *crypto.RefreshService

	// 数据管理器
	DataManager *data_management.DataManager

	// 数据打包服务
	DataPackingService *data_management.DataPackingService

	// 层处理器
	LayerProcessor interfaces.LayerProcessor

	// 同步服务
	SyncService sync.SyncService

	// 端口
	Port int

	// 状态管理
	Ready   bool
	ReadyCh chan struct{}

	// 输出缓冲区
	OutputBuffer *buffer.OutputBuffer
}

// NewParticipant 创建新的参与方实例
func NewParticipant() *Participant {
	client := &types.HTTPClient{
		Client: &http.Client{Timeout: 300 * time.Second}, // 增加到5分钟，给密钥传输留出足够时间
	}

	// 创建密钥管理器
	keyManager := crypto.NewKeyManager()

	// 创建解密服务
	decryptionService := crypto.NewDecryptionService(keyManager, client)

	// 创建刷新服务
	refreshService := crypto.NewRefreshService(keyManager, client)

	participant := &Participant{
		Client:            client,
		KeyManager:        keyManager,
		DecryptionService: decryptionService,
		RefreshService:    refreshService,
		PeerManager:       network.NewPeerManager(),
		ReadyCh:           make(chan struct{}),
	}

	// 初始化服务组件
	participant.DataManager = data_management.NewDataManager()
	participant.MessageProcessor = NewMessageProcessor(participant)
	participant.RegistryService = NewRegistryService(participant)

	// 初始化同步服务（暂时为nil，在设置参与方信息后初始化）
	participant.SyncService = nil

	// 初始化层处理器
	// participant.InputLayerProcessor = input_layer.NewInputLayerProcessor(participant)
	// participant.HiddenLayerProcessor = hidden_layer.NewHiddenLayerProcessor(participant)
	// participant.OutputLayerProcessor = output_layer.NewOutputLayerProcessor(participant)

	// 数据打包服务将在获取参数后初始化
	participant.DataPackingService = nil

	// 初始化输出缓冲区
	participant.OutputBuffer = buffer.NewOutputBuffer("participant", 1000)

	return participant
}

// Register 注册到协调器并启动P2P服务器
func (p *Participant) Register(coordinatorURL string) error {
	return p.RegistryService.Register(coordinatorURL)
}

// UpdateOnlineParticipants 更新在线参与方列表
func (p *Participant) UpdateOnlineParticipants() error {
	return p.RegistryService.UpdateOnlineParticipants()
}

// GetParams 获取参数
func (p *Participant) GetParams() error {
	// 暂时使用默认参数，后续可以从协调器获取
	p.LogInfo("使用默认CKKS参数初始化数据打包服务")

	// 使用默认参数初始化数据打包服务
	p.initializeDataPackingService()

	return nil
}

// initializeDataPackingService 初始化数据打包服务
func (p *Participant) initializeDataPackingService() {
	// 暂时使用默认参数，后续可以从协调器获取
	p.LogWarning("使用默认参数，数据打包服务功能受限")

	// 推荐参数配置
	slotCount := 8192  // CKKS slot数
	featureCount := 64 // 每图像特征数k

	// 创建数据打包服务（暂时不传入加密器）
	p.DataPackingService = data_management.NewDataPackingService(slotCount, featureCount, ckks.Parameters{}, nil)

	// 打印配置信息
	p.DataPackingService.PrintConfiguration()
}

// UpdateDataPackingServiceWithKeys 在获取密钥后更新数据打包服务
func (p *Participant) UpdateDataPackingServiceWithKeys() error {
	if !p.KeyManager.IsReady() {
		return fmt.Errorf("密钥管理器未准备就绪")
	}

	params := p.KeyManager.GetParams()
	encryptor := p.KeyManager.GetEncryptor()
	encoder := p.KeyManager.GetEncoder()

	if encryptor == nil || encoder == nil {
		return fmt.Errorf("无法获取加密器或编码器")
	}

	// 推荐参数配置
	slotCount := 8192  // CKKS slot数
	featureCount := 64 // 每图像特征数k

	// 重新创建数据打包服务，包含完整的加密功能
	p.DataPackingService = data_management.NewDataPackingService(slotCount, featureCount, params, encryptor)

	// 更新编码器引用
	p.DataPackingService.Encoder = encoder

	p.LogInfo("✓ 数据打包服务已更新，加密功能已启用")
	p.DataPackingService.PrintConfiguration()

	return nil
}

// ShowOnlineStatus 显示在线状态
func (p *Participant) ShowOnlineStatus() error {
	return p.HeartbeatManager.ShowOnlineStatus()
}

// CheckOnlineStatusBeforeOperation 在操作前检查在线状态
func (p *Participant) CheckOnlineStatusBeforeOperation() error {
	return p.HeartbeatManager.CheckOnlineStatusBeforeOperation()
}

// SetSilentMode 设置静默模式
func (p *Participant) SetSilentMode(silent bool) {
	p.HeartbeatManager.SetSilentMode(silent)
}

// StopHeartbeat 停止心跳
func (p *Participant) StopHeartbeat() {
	p.HeartbeatManager.StopHeartbeat()
}

// RequestCollaborativeDecrypt 发起协同解密请求
func (p *Participant) RequestCollaborativeDecrypt() error {
	onlinePeers := p.HeartbeatManager.GetOnlinePeers()
	return p.DecryptionService.RequestCollaborativeDecrypt(onlinePeers, p.ID)
}

// RequestCollaborativeRefresh 发起协同刷新请求
func (p *Participant) RequestCollaborativeRefresh() error {
	onlinePeers := p.HeartbeatManager.GetOnlinePeers()
	return p.RefreshService.RequestCollaborativeRefresh(onlinePeers, p.ID)
}

// RunMainLoop 运行主循环
func (p *Participant) RunMainLoop() {
	// 进入菜单模式，启用静默模式
	p.SetSilentMode(true)
	for {
		fmt.Println("\n请选择操作：")
		fmt.Println("1. 发起协同解密请求")
		fmt.Println("2. 发起协同刷新请求")
		fmt.Println("3. 查看在线状态")
		fmt.Println("4. 退出")
		fmt.Print("输入选项: ")

		var choice int
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("输入无效，请重新输入。")
			continue
		}

		switch choice {
		case 1:
			// 先检查在线状态
			if err := p.CheckOnlineStatusBeforeOperation(); err != nil {
				fmt.Println("[错误] 在线状态检查失败:", err)
				continue
			}
			// 发起协同解密请求
			if err := p.RequestCollaborativeDecrypt(); err != nil {
				fmt.Printf("[错误] 协同解密失败: %v\n", err)
			}
			continue
		case 2:
			// 先检查在线状态
			if err := p.CheckOnlineStatusBeforeOperation(); err != nil {
				fmt.Println("[错误] 在线状态检查失败:", err)
				continue
			}
			// 发起协同刷新请求
			if err := p.RequestCollaborativeRefresh(); err != nil {
				fmt.Printf("[错误] 协同刷新失败: %v\n", err)
			}
			continue
		case 3:
			// 临时禁用静默模式以显示状态
			p.SetSilentMode(false)
			if err := p.ShowOnlineStatus(); err != nil {
				fmt.Println("[错误] 获取在线状态失败:", err)
			}
			// 重新启用静默模式
			p.SetSilentMode(true)
			continue
		case 4:
			fmt.Println("退出程序。")
			p.StopHeartbeat()
			return
		default:
			fmt.Println("无效选项，请重新输入。")
		}
	}
}

// GetOnlineParticipants 获取在线参与方列表
func (p *Participant) GetOnlineParticipants() map[int]string {
	// 优先使用心跳管理器获取最新的在线参与方信息
	if p.HeartbeatManager != nil {
		return p.HeartbeatManager.GetOnlinePeers()
	}
	// 回退到PeerManager
	return p.PeerManager.GetPeers()
}

// RefreshOnlineParticipants 刷新在线参与方列表
func (p *Participant) RefreshOnlineParticipants() error {
	return p.UpdateOnlineParticipants()
}

// GetID 获取参与方ID
func (p *Participant) GetID() int {
	return p.ID
}

// GetDataSplit 获取数据划分方式
func (p *Participant) GetDataSplit() string {
	return p.DataManager.GetDataSplit()
}

// HandleMessage 处理来自其他参与方的消息
func (p *Participant) HandleMessage(fromID int, message string) {
	p.MessageProcessor.HandleMessage(fromID, message)
}

// SendMessageToParticipant 向指定参与方发送消息
func (p *Participant) SendMessageToParticipant(targetID int, message string) error {
	return p.MessageProcessor.SendMessageToParticipant(targetID, message)
}

// GenerateAllCRPs 根据参数生成所有CRP
func (p *Participant) GenerateAllCRPs(paramsResp *types.ParamsResponse) error {
	// 创建CKKS参数
	params, err := ckks.NewParametersFromLiteral(paramsResp.Params)
	if err != nil {
		return fmt.Errorf("创建CKKS参数失败: %v", err)
	}

	// 使用从协调器接收的统一CRS种子
	commonCRSSeedBytes, err := utils.DecodeFromBase64(paramsResp.CommonCRSSeed)
	if err != nil {
		return fmt.Errorf("解码统一CRS种子失败: %v", err)
	}

	// 使用统一种子创建PRNG
	crs, err := sampling.NewKeyedPRNG(commonCRSSeedBytes)
	if err != nil {
		return fmt.Errorf("使用统一种子创建PRNG失败: %v", err)
	}

	// 生成公钥CRP
	pubKeyProto := multiparty.NewPublicKeyGenProtocol(params)
	pubKeyCRP := pubKeyProto.SampleCRP(crs)
	pubKeyCRPRaw, err := utils.EncodeShare(pubKeyCRP)
	if err != nil {
		return fmt.Errorf("编码公钥CRP失败: %v", err)
	}
	paramsResp.Crp = utils.EncodeToBase64(pubKeyCRPRaw)

	// 使用协调器传输的伽罗瓦元素（避免重复生成）
	galoisProto := multiparty.NewGaloisKeyGenProtocol(params)
	galoisCRPs := make(map[uint64]string)
	for _, galEl := range paramsResp.GalEls {
		galoisCRP := galoisProto.SampleCRP(crs)
		crpRaw, err := utils.EncodeShare(galoisCRP)
		if err != nil {
			return fmt.Errorf("编码伽罗瓦CRP失败: %v", err)
		}
		galoisCRPs[galEl] = utils.EncodeToBase64(crpRaw)
	}
	paramsResp.GaloisCRPs = galoisCRPs

	// 生成重线性化密钥CRP
	rlkProto := multiparty.NewRelinearizationKeyGenProtocol(params)
	rlkCRP := rlkProto.SampleCRP(crs)
	rlkCRPRaw, err := utils.EncodeShare(rlkCRP)
	if err != nil {
		return fmt.Errorf("编码重线性化CRP失败: %v", err)
	}
	paramsResp.RlkCRP = utils.EncodeToBase64(rlkCRPRaw)

	// 生成刷新CRS
	refreshCRSSeed := []byte("refresh_crs_seed_32_bytes_long")
	paramsResp.RefreshCRS = utils.EncodeToBase64(refreshCRSSeed)

	// 设置参数到KeyManager
	p.KeyManager.SetParams(params)

	fmt.Printf("参与方 %d 生成了所有CRP：公钥CRP、%d个伽罗瓦CRP、重线性化CRP、刷新CRS\n", p.ID, len(galoisCRPs))
	fmt.Printf("✓ 参数已设置到密钥管理器\n")
	return nil
}

// Unregister 注销参与方
func (p *Participant) Unregister() error {
	return p.RegistryService.Unregister()
}

// DetermineRole 根据参与方ID判定角色
func (p *Participant) DetermineRole() error {
	// 从协调器获取详细状态，包含总参与方数量
	detailedStatus, err := p.CoordinatorClient.GetDetailedStatus()
	if err != nil {
		return fmt.Errorf("获取协调器详细状态失败: %v", err)
	}

	// 设置总参与方数量
	p.DataManager.SetTotalParticipants(detailedStatus.TotalParticipants)

	// 根据参与方ID和总参与方数量判定角色
	p.DataManager.DetermineRole(p.ID)

	// 输出角色信息
	fmt.Printf("参与方 %d 角色判定完成:\n", p.ID)
	fmt.Printf("  总参与方数量: %d\n", detailedStatus.TotalParticipants)
	fmt.Printf("  当前角色: %s (%s)\n", p.DataManager.GetRole(), p.DataManager.GetRoleDescription())
	if p.DataManager.IsTestMode() {
		fmt.Printf("  运行模式: 测试模式\n")
	} else {
		fmt.Printf("  运行模式: 正常模式\n")
	}

	return nil
}

// GetSyncService 获取同步服务
func (p *Participant) GetSyncService() sync.SyncService {
	return p.SyncService
}

// GetRole 获取当前角色
func (p *Participant) GetRole() string {
	return p.DataManager.GetRole()
}

// IsInputLayer 检查是否为输入层
func (p *Participant) IsInputLayer() bool {
	return p.DataManager.IsInputLayer()
}

// IsHiddenLayer 检查是否为隐藏层
func (p *Participant) IsHiddenLayer() bool {
	return p.DataManager.IsHiddenLayer()
}

// IsOutputLayer 检查是否为输出层
func (p *Participant) IsOutputLayer() bool {
	return p.DataManager.IsOutputLayer()
}

// GetInputLayerProcessor 获取输入层处理器
func (p *Participant) GetInputLayerProcessor() (interfaces.FeatureCollector, bool) {
	if inputProcessor, ok := p.LayerProcessor.(interfaces.FeatureCollector); ok {
		return inputProcessor, true
	}
	return nil, false
}

// GetOutputLayerProcessor 获取输出层处理器
func (p *Participant) GetOutputLayerProcessor() (interfaces.LabelCollector, bool) {
	if outputProcessor, ok := p.LayerProcessor.(interfaces.LabelCollector); ok {
		return outputProcessor, true
	}
	return nil, false
}

// GetOutputBuffer 获取输出缓冲区
func (p *Participant) GetOutputBuffer() *buffer.OutputBuffer {
	return p.OutputBuffer
}

// ProcessDatasetWithPacking 根据角色处理数据集重排打包加密
func (p *Participant) ProcessDatasetWithPacking() error {
	if p.DataPackingService == nil {
		return fmt.Errorf("数据打包服务未初始化")
	}

	role := p.GetRole()
	p.LogOutputf("\n=== 参与方 %d (%s) 数据集处理开始 ===\n", p.ID, role)

	// 获取数据集
	images := p.DataManager.GetImages()
	if len(images) == 0 {
		return fmt.Errorf("数据集为空")
	}

	p.LogOutputf("数据集信息:\n")
	p.LogOutputf("  • 图像数量: %d\n", len(images))
	p.LogOutputf("  • 每图像特征数: %d\n", len(images[0]))
	p.LogOutputf("  • 数据划分方式: %s\n", p.DataManager.GetDataSplit())

	// 根据角色设置对应的层处理器
	switch role {
	case "input_layer":
		inputProcessor := input_layer.NewInputLayerProcessor(p.DataPackingService, p.DataManager, p.ID)
		inputProcessor.SetParticipantInfo(p.PeerManager, p.MessageProcessor)
		// 设置密钥管理器
		inputProcessor.SetKeyManager(p.KeyManager)
		p.LayerProcessor = inputProcessor
		// 初始化同步服务
		p.SyncService = sync.NewSyncService(p.ID, p.PeerManager, p.Client.Client)
	case "hidden_layer":
		// 创建权重管理器（修复参数传递）
		weightManager := NewWeightManager(p.DataPackingService.GetSlotCount(), p.DataPackingService.GetFeatureCount(), p.DataPackingService.GetParams(), p.DataPackingService.GetEncryptor())
		weightManager.SetEncoder(p.DataPackingService.GetEncoder())

		hiddenProcessor := hidden_layer.NewHiddenLayerProcessor(p.DataPackingService, p.DataManager, p.ID, weightManager)
		hiddenProcessor.SetParticipantInfo(p.PeerManager, p.MessageProcessor)
		// 设置密钥管理器
		hiddenProcessor.SetKeyManager(p.KeyManager)
		p.LayerProcessor = hiddenProcessor
		// 初始化同步服务
		p.SyncService = sync.NewSyncService(p.ID, p.PeerManager, p.Client.Client)
	case "output_layer":
		// 创建权重管理器（修复参数传递）
		weightManager := NewWeightManager(p.DataPackingService.GetSlotCount(), p.DataPackingService.GetFeatureCount(), p.DataPackingService.GetParams(), p.DataPackingService.GetEncryptor())
		weightManager.SetEncoder(p.DataPackingService.GetEncoder())

		outputProcessor := output_layer.NewOutputLayerProcessor(p.DataPackingService, p.DataManager, p.ID, weightManager)
		outputProcessor.SetParticipantInfo(p.PeerManager, p.MessageProcessor)
		// 设置密钥管理器
		outputProcessor.SetKeyManager(p.KeyManager)
		// 设置刷新服务
		if p.RefreshService != nil {
			outputProcessor.SetRefreshService(p.RefreshService)
		}
		// 设置网络信息
		onlinePeers := p.GetOnlineParticipants()
		outputProcessor.SetNetworkInfo(onlinePeers, p.ID)
		// 设置协调器客户端
		if p.CoordinatorClient != nil {
			outputProcessor.SetCoordinatorClient(p.CoordinatorClient)
		}
		p.LayerProcessor = outputProcessor
		// 初始化同步服务
		p.SyncService = sync.NewSyncService(p.ID, p.PeerManager, p.Client.Client)
	case "input_output_layer":
		// 测试模式：使用输出层处理器（包含标签处理功能）
		// 创建权重管理器（修复参数传递）
		weightManager := NewWeightManager(p.DataPackingService.GetSlotCount(), p.DataPackingService.GetFeatureCount(), p.DataPackingService.GetParams(), p.DataPackingService.GetEncryptor())
		weightManager.SetEncoder(p.DataPackingService.GetEncoder())

		outputProcessor := output_layer.NewOutputLayerProcessor(p.DataPackingService, p.DataManager, p.ID, weightManager)
		outputProcessor.SetParticipantInfo(p.PeerManager, p.MessageProcessor)
		// 设置密钥管理器
		outputProcessor.SetKeyManager(p.KeyManager)
		// 设置刷新服务
		if p.RefreshService != nil {
			outputProcessor.SetRefreshService(p.RefreshService)
		}
		// 设置网络信息
		onlinePeers := p.GetOnlineParticipants()
		outputProcessor.SetNetworkInfo(onlinePeers, p.ID)
		// 设置协调器客户端
		if p.CoordinatorClient != nil {
			outputProcessor.SetCoordinatorClient(p.CoordinatorClient)
		}
		p.LayerProcessor = outputProcessor
		// 初始化同步服务
		p.SyncService = sync.NewSyncService(p.ID, p.PeerManager, p.Client.Client)
	default:
		return fmt.Errorf("未知角色: %s", role)
	}

	// 使用层处理器处理数据
	if err := p.LayerProcessor.ProcessData(images); err != nil {
		return fmt.Errorf("层数据处理失败: %v", err)
	}

	// 根据角色执行特征收集/发送和标签收集/发送
	switch role {
	case "input_layer":
		// Input Layer: 收集所有参与方的特征数据，发送标签数据给Output Layer，发送收集好的特征数据给Hidden Layer
		if inputProcessor, ok := p.LayerProcessor.(interfaces.FeatureCollector); ok {
			if err := inputProcessor.CollectAllFeatures(); err != nil {
				return fmt.Errorf("特征收集失败: %v", err)
			}
			// 组织特征数据为神经网络输入格式
			organizedFeatures := inputProcessor.OrganizeFeatures()
			p.LogOutputf("✓ 特征数据组织完成，准备开始神经网络计算\n")
			p.LogOutputf("  收集到的参与方: %d 个\n", len(organizedFeatures))
		}

		// 1. 先发送标签数据给Output Layer
		p.LogOutputf("  检查输入层处理器是否实现LabelSender接口...\n")
		if inputProcessor, ok := p.LayerProcessor.(interfaces.LabelSender); ok {
			p.LogOutputf("  ✓ 类型断言成功，开始发送标签数据给输出层\n")
			if err := inputProcessor.SendLabelsToOutputLayer(); err != nil {
				return fmt.Errorf("标签发送失败: %v", err)
			}
		} else {
			p.LogOutputf("  ❌ 类型断言失败，LayerProcessor类型: %T\n", p.LayerProcessor)
			p.LogOutputf("  ❌ 输入层处理器未实现LabelSender接口\n")
		}

		// 2. 再发送收集好的特征数据给Hidden Layer
		p.LogOutputf("  开始发送收集好的特征数据给隐藏层...\n")
		// 使用类型断言获取具体的输入层处理器
		if concreteInputProcessor, ok := p.LayerProcessor.(*input_layer.InputLayerProcessor); ok {
			if err := concreteInputProcessor.SendFeaturesToHiddenLayer(0, 2); err != nil { // 批次0，目标参与方2（隐藏层）
				return fmt.Errorf("发送特征数据给隐藏层失败: %v", err)
			}
			p.LogOutputf("✓ 特征数据发送给隐藏层完成\n")
		} else {
			return fmt.Errorf("无法获取具体的输入层处理器")
		}
	case "hidden_layer":
		// Hidden Layer: 向输入层发送密文特征、输出层发送密文标签、等待接收输入层特征数据
		p.LogOutputf("  隐藏层开始发送特征和标签数据...\n")

		// 1. 向输入层发送密文特征
		if hiddenProcessor, ok := p.LayerProcessor.(interfaces.FeatureSender); ok {
			if err := hiddenProcessor.SendFeaturesToInputLayer(); err != nil {
				return fmt.Errorf("特征发送失败: %v", err)
			}
		}

		// 2. 向输出层发送密文标签
		if hiddenProcessor, ok := p.LayerProcessor.(interfaces.LabelSender); ok {
			if err := hiddenProcessor.SendLabelsToOutputLayer(); err != nil {
				return fmt.Errorf("标签发送失败: %v", err)
			}
		}

		// 3. 等待接收输入层特征数据
		p.LogOutputf("  隐藏层等待接收输入层的特征数据...\n")
		// 注意：隐藏层会通过MessageProcessor接收输入层发送的特征数据
	case "output_layer":
		// Output Layer: 发送特征数据给Input Layer，收集所有参与方的标签数据
		p.LogOutputf("  输出层开始发送特征数据...\n")

		// 1. 发送特征数据给Input Layer
		if outputProcessor, ok := p.LayerProcessor.(interfaces.FeatureSender); ok {
			if err := outputProcessor.SendFeaturesToInputLayer(); err != nil {
				return fmt.Errorf("特征发送失败: %v", err)
			}
		}

		// 2. 收集所有参与方的标签数据
		if outputProcessor, ok := p.LayerProcessor.(interfaces.LabelCollector); ok {
			if err := outputProcessor.CollectAllLabels(); err != nil {
				return fmt.Errorf("标签收集失败: %v", err)
			}
			// 组织标签数据为损失计算格式
			organizedLabels := outputProcessor.OrganizeLabels()
			p.LogOutputf("✓ 标签数据组织完成，准备开始损失计算\n")
			p.LogOutputf("  收集到的参与方: %d 个\n", len(organizedLabels))
		}
	case "input_output_layer":
		// 测试模式：同时测试特征发送和标签收集
		labels := p.DataManager.GetLabels()
		if len(labels) > 0 {
			// 使用类型断言获取输出层处理器
			if outputProcessor, ok := p.LayerProcessor.(*output_layer.OutputLayerProcessor); ok {
				if err := outputProcessor.ProcessLabelData(labels); err != nil {
					return fmt.Errorf("标签数据处理失败: %v", err)
				}
				if err := outputProcessor.VerifyEncryptionCorrectness(images, labels); err != nil {
					return fmt.Errorf("加密正确性验证失败: %v", err)
				}
			}
		}
		// 发送特征数据（模拟Input Layer）
		if outputProcessor, ok := p.LayerProcessor.(interfaces.FeatureSender); ok {
			if err := outputProcessor.SendFeaturesToInputLayer(); err != nil {
				return fmt.Errorf("特征发送失败: %v", err)
			}
		}
		// 收集标签数据（模拟Output Layer）
		if outputProcessor, ok := p.LayerProcessor.(interfaces.LabelCollector); ok {
			if err := outputProcessor.CollectAllLabels(); err != nil {
				return fmt.Errorf("标签收集失败: %v", err)
			}
			organizedLabels := outputProcessor.OrganizeLabels()
			p.LogOutputf("✓ 测试模式：标签数据组织完成，共 %d 个参与方\n", len(organizedLabels))
		}
	}

	return nil
}

// LogOutput 同时输出到控制台和输出缓冲区
func (p *Participant) LogOutput(level, message string) {
	// 输出到控制台
	switch level {
	case "info":
		fmt.Printf("[INFO] %s\n", message)
	case "warning":
		fmt.Printf("[WARNING] %s\n", message)
	case "error":
		fmt.Printf("[ERROR] %s\n", message)
	case "debug":
		fmt.Printf("[DEBUG] %s\n", message)
	default:
		fmt.Printf("[%s] %s\n", strings.ToUpper(level), message)
	}

	// 记录到输出缓冲区
	if p.OutputBuffer != nil {
		p.OutputBuffer.AddOutput(level, message)
	}
}

// LogInfo 记录信息级别日志
func (p *Participant) LogInfo(message string) {
	p.LogOutput("info", message)
}

// LogWarning 记录警告级别日志
func (p *Participant) LogWarning(message string) {
	p.LogOutput("warning", message)
}

// LogError 记录错误级别日志
func (p *Participant) LogError(message string) {
	p.LogOutput("error", message)
}

// LogDebug 记录调试级别日志
func (p *Participant) LogDebug(message string) {
	p.LogOutput("debug", message)
}

// LogOutputf 记录格式化的普通输出（同时输出到控制台和OutputBuffer）
func (p *Participant) LogOutputf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Print(message)
	if p.OutputBuffer != nil {
		p.OutputBuffer.AddInfo(message)
	}
}
