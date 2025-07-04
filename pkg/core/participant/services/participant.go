package services

import (
	"MPHEDev/pkg/core/participant/coordinator"
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/network"
	"MPHEDev/pkg/core/participant/server"
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// Participant 重构后的参与方主结构体
type Participant struct {
	ID     int
	Port   int
	Client *types.HTTPClient

	// 网络相关
	PeerManager      *network.PeerManager
	HeartbeatManager *network.HeartbeatManager
	HTTPServer       *server.HTTPServer

	// 协调器客户端
	CoordinatorClient *coordinator.CoordinatorClient

	// 加密相关
	KeyManager        *crypto.KeyManager
	DecryptionService *crypto.DecryptionService
	RefreshService    *crypto.RefreshService

	// 状态管理
	Ready   bool
	ReadyCh chan struct{}

	// 数据集相关
	Images    [][]float64 // 载入的图像数据
	Labels    []int       // 载入的标签数据
	DataSplit string      // 数据分片类型：vertical 或 horizontal

	// 数据分发状态
	ReceivedFeatures     map[int]bool // 已接收特征数据的参与方ID
	ReceivedLabels       map[int]bool // 已接收标签数据的参与方ID
	DataDistributionDone bool         // 数据分发是否完成

	// 输入层和输出层Done状态
	InputLayerDone  bool // 输入层数据分发完成
	OutputLayerDone bool // 输出层数据分发完成

	// 分批接收状态管理
	FeatureBatchStatus map[int]*BatchStatus // 每个参与方的特征批次状态
	LabelBatchStatus   map[int]*BatchStatus // 每个参与方的标签批次状态

	// 接收到的密文数据存储
	ReceivedFeatureCiphertexts map[int][][]string // 每个参与方的特征密文批次
	ReceivedLabelCiphertexts   map[int][][]string // 每个参与方的标签密文批次
}

// BatchStatus 批次状态
type BatchStatus struct {
	TotalBatches    int          // 总批次数
	ReceivedBatches map[int]bool // 已接收的批次
	AllReceived     bool         // 是否全部接收完成
}

// NewParticipant 创建新的参与方实例
func NewParticipant() *Participant {
	client := &types.HTTPClient{
		Client: &http.Client{Timeout: 60 * time.Second},
	}

	// 创建密钥管理器
	keyManager := crypto.NewKeyManager()

	// 创建解密服务
	decryptionService := crypto.NewDecryptionService(keyManager, client)

	// 创建刷新服务
	refreshService := crypto.NewRefreshService(keyManager, client)

	return &Participant{
		Client:                     client,
		KeyManager:                 keyManager,
		DecryptionService:          decryptionService,
		RefreshService:             refreshService,
		ReadyCh:                    make(chan struct{}),
		ReceivedFeatures:           make(map[int]bool),
		ReceivedLabels:             make(map[int]bool),
		DataDistributionDone:       false,
		InputLayerDone:             false,
		OutputLayerDone:            false,
		FeatureBatchStatus:         make(map[int]*BatchStatus),
		LabelBatchStatus:           make(map[int]*BatchStatus),
		ReceivedFeatureCiphertexts: make(map[int][][]string),
		ReceivedLabelCiphertexts:   make(map[int][][]string),
	}
}

func getLocalShardID(dataSplit string) string {
	var dataDir string
	if dataSplit == "vertical" {
		dataDir = "../../data/vertical"
	} else {
		dataDir = "../../data/horizontal"
	}
	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		fmt.Printf("读取目录失败: %s, err=%v\n", dataDir, err)
		return ""
	}
	fmt.Printf("扫描目录 %s，文件列表：\n", dataDir)
	for _, f := range files {
		fmt.Println("  ", f.Name())
		if strings.HasPrefix(f.Name(), "train_split_") && strings.HasSuffix(f.Name(), "_images.csv") {
			parts := strings.Split(f.Name(), "_")
			if len(parts) >= 3 {
				fmt.Printf("检测到分片文件: %s，分片ID: %s\n", f.Name(), parts[2])
				return parts[2] // 000、001等
			}
		}
	}
	return ""
}

// autoDetectDataSplit 自动检测数据分片类型
func autoDetectDataSplit() string {
	if files, err := ioutil.ReadDir("../../data/vertical"); err == nil && len(files) > 0 {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "train_split_") && strings.HasSuffix(f.Name(), "_images.csv") {
				return "vertical"
			}
		}
	}
	if files, err := ioutil.ReadDir("../../data/horizontal"); err == nil && len(files) > 0 {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "train_split_") && strings.HasSuffix(f.Name(), "_images.csv") {
				return "horizontal"
			}
		}
	}
	return ""
}

// Register 注册到协调器并启动P2P服务器
func (p *Participant) Register(coordinatorURL string) error {
	// 1. 自动检测数据集划分方式
	dataSplit := autoDetectDataSplit()
	if dataSplit == "" {
		return fmt.Errorf("未找到本地分片文件，无法注册")
	}
	p.DataSplit = dataSplit
	shardID := getLocalShardID(dataSplit)
	if shardID == "" {
		return fmt.Errorf("未找到本地分片文件，无法注册")
	}
	// 2. 创建协调器客户端
	p.CoordinatorClient = coordinator.NewCoordinatorClient(coordinatorURL, p.Client)
	// 3. 注册获取ID
	regResp, err := p.CoordinatorClient.Register(shardID)
	if err != nil {
		return fmt.Errorf("注册失败: %v", err)
	}
	p.ID = regResp.ParticipantID

	// 4. 设置端口
	p.Port = 8081 // 使用固定端口8081，因为不同机器上运行

	// 5. 创建P2P网络管理器
	p.PeerManager = network.NewPeerManager()

	// 6. 创建心跳管理器
	p.HeartbeatManager = network.NewHeartbeatManager(coordinatorURL, p.Client, p.ID)

	// 7. 启动P2P服务器
	if err := p.startHTTPServer(); err != nil {
		return fmt.Errorf("启动P2P服务器失败: %v", err)
	}

	// 8. 等待服务器启动
	time.Sleep(1 * time.Second)

	// 9. 向协调器上报自己的URL
	if err := p.PeerManager.ReportURL(coordinatorURL, p.Client, p.ID, p.Port); err != nil {
		return fmt.Errorf("上报URL失败: %v", err)
	}

	// 10. 获取其他参与方的URL
	if err := p.PeerManager.DiscoverPeers(coordinatorURL, p.Client, p.ID); err != nil {
		return fmt.Errorf("发现其他参与方失败: %v", err)
	}

	// 11. 启动心跳机制
	p.HeartbeatManager.Start()

	// 12. 启动在线状态监控
	p.HeartbeatManager.StartOnlineStatusMonitor()

	// 13. 发送初始心跳，确保自己能被识别为在线
	if err := p.HeartbeatManager.SendInitialHeartbeat(); err != nil {
		return fmt.Errorf("发送初始心跳失败: %v", err)
	}

	// 14. 获取参数并设置数据集划分方式
	paramsResp, err := p.CoordinatorClient.GetParams()
	if err != nil {
		return fmt.Errorf("获取参数失败: %v", err)
	}

	// 设置数据集划分方式
	p.DataSplit = paramsResp.DataSplitType

	// 15. 获取在线成员列表
	if err := p.UpdateOnlineParticipants(); err != nil {
		return fmt.Errorf("获取在线成员列表失败: %v", err)
	}

	return nil
}

// UpdateOnlineParticipants 更新在线参与方列表
func (p *Participant) UpdateOnlineParticipants() error {
	// 从协调器获取在线参与方列表
	onlineParticipants := p.HeartbeatManager.GetOnlinePeers()

	// 更新P2P网络管理器中的参与方列表
	for id, url := range onlineParticipants {
		if id != p.ID { // 不添加自己
			p.PeerManager.AddPeer(id, url)
		}
	}

	// 统计时包含自己
	totalOnline := len(onlineParticipants) + 1 // 加上自己
	fmt.Printf("当前在线参与方: %d 个 (包括自己)\n", totalOnline)
	fmt.Printf("  参与方 %d: %s (自己)\n", p.ID, p.HTTPServer.GetLocalIP())
	for id, url := range onlineParticipants {
		fmt.Printf("  参与方 %d: %s\n", id, url)
	}

	return nil
}

// startHTTPServer 启动HTTP服务器
func (p *Participant) startHTTPServer() error {
	// 创建HTTP处理器
	handlers := server.NewHandlers(p.KeyManager, p.DecryptionService, p.RefreshService)
	handlerMap := handlers.GetHandlers()

	// 创建HTTP服务器
	p.HTTPServer = server.NewHTTPServer(p.Port, handlerMap, p)

	// 启动服务器
	return p.HTTPServer.Start()
}

// GetParams 获取参数
func (p *Participant) GetParams() error {
	// 这里需要实现获取参数的逻辑
	// 暂时返回nil
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
	return p.DataSplit
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

	fmt.Printf("参与方 %d 生成了所有CRP：公钥CRP、%d个伽罗瓦CRP、重线性化CRP、刷新CRS\n", p.ID, len(galoisCRPs))
	return nil
}

// Unregister 注销参与方
func (p *Participant) Unregister() error {
	shardID := getLocalShardID(p.DataSplit)
	if shardID == "" {
		return fmt.Errorf("未找到本地分片文件，无法注销")
	}
	return p.CoordinatorClient.Unregister(shardID)
}

// DataMessage 数据消息结构
type DataMessage struct {
	Type      string   `json:"type"` // "feature", "label", "done", "input_done", "output_done"
	From      int      `json:"from"`
	Data      string   `json:"data,omitempty"`       // base64编码的密文
	BatchData []string `json:"batch_data,omitempty"` // base64编码的密文批次
}
