package services

import (
	"MPHEDev/cmd/Participant/coordinator"
	"MPHEDev/cmd/Participant/crypto"
	"MPHEDev/cmd/Participant/network"
	"MPHEDev/cmd/Participant/server"
	"MPHEDev/cmd/Participant/types"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
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
}

// NewParticipant 创建新的参与方实例
func NewParticipant() *Participant {
	client := &types.HTTPClient{
		Client: &http.Client{Timeout: 30 * time.Second},
	}

	// 创建密钥管理器
	keyManager := crypto.NewKeyManager()

	// 创建解密服务
	decryptionService := crypto.NewDecryptionService(keyManager, client)

	// 创建刷新服务
	refreshService := crypto.NewRefreshService(keyManager, client)

	return &Participant{
		Client:            client,
		KeyManager:        keyManager,
		DecryptionService: decryptionService,
		RefreshService:    refreshService,
		ReadyCh:           make(chan struct{}),
	}
}

// Register 注册到协调器并启动P2P服务器
func (p *Participant) Register(coordinatorURL string) error {
	// 1. 创建协调器客户端
	p.CoordinatorClient = coordinator.NewCoordinatorClient(coordinatorURL, p.Client)

	// 2. 注册获取ID
	regResp, err := p.CoordinatorClient.Register()
	if err != nil {
		return fmt.Errorf("注册失败: %v", err)
	}
	p.ID = regResp.ParticipantID

	// 3. 设置端口
	p.Port = 8080 + p.ID

	// 4. 创建P2P网络管理器
	p.PeerManager = network.NewPeerManager()

	// 5. 创建心跳管理器
	p.HeartbeatManager = network.NewHeartbeatManager(coordinatorURL, p.Client)

	// 6. 启动P2P服务器
	if err := p.startHTTPServer(); err != nil {
		return fmt.Errorf("启动P2P服务器失败: %v", err)
	}

	// 7. 等待服务器启动
	time.Sleep(1 * time.Second)

	// 8. 向协调器上报自己的URL
	if err := p.PeerManager.ReportURL(coordinatorURL, p.Client, p.ID, p.Port); err != nil {
		return fmt.Errorf("上报URL失败: %v", err)
	}

	// 9. 获取其他参与方的URL
	if err := p.PeerManager.DiscoverPeers(coordinatorURL, p.Client, p.ID); err != nil {
		return fmt.Errorf("发现其他参与方失败: %v", err)
	}

	// 10. 启动心跳机制
	p.HeartbeatManager.StartHeartbeat(p.ID)

	// 11. 启动在线状态监控
	p.HeartbeatManager.StartOnlineStatusMonitor()

	// 12. 获取参数并设置数据集划分方式
	paramsResp, err := p.CoordinatorClient.GetParams()
	if err != nil {
		return fmt.Errorf("获取参数失败: %v", err)
	}

	// 设置数据集划分方式
	p.DataSplit = paramsResp.DataSplitType
	fmt.Printf("参与方 %d 接收到数据集划分方式: %s\n", p.ID, p.DataSplit)

	// 13. 载入本地数据集
	if err := p.LoadDataset(); err != nil {
		return fmt.Errorf("载入数据集失败: %v", err)
	}

	return nil
}

// startHTTPServer 启动HTTP服务器
func (p *Participant) startHTTPServer() error {
	// 创建HTTP处理器
	handlers := server.NewHandlers(p.KeyManager, p.DecryptionService, p.RefreshService)
	handlerMap := handlers.GetHandlers()

	// 创建HTTP服务器
	p.HTTPServer = server.NewHTTPServer(p.Port, handlerMap)

	// 启动服务器
	return p.HTTPServer.Start(p.ID)
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
	// 等待密钥分发完成
	fmt.Println("等待密钥分发完成...")
	<-p.ReadyCh
	fmt.Println("密钥分发完成，进入菜单模式！")

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
		case 3:
			// 临时禁用静默模式以显示状态
			p.SetSilentMode(false)
			if err := p.ShowOnlineStatus(); err != nil {
				fmt.Println("[错误] 获取在线状态失败:", err)
			}
			// 重新启用静默模式
			p.SetSilentMode(true)
		case 4:
			fmt.Println("退出程序。")
			p.StopHeartbeat()
			return
		default:
			fmt.Println("无效选项，请重新输入。")
		}
	}
}

// LoadDataset 载入本地数据集分片
func (p *Participant) LoadDataset() error {
	// 构建数据目录路径 - 相对于项目根目录
	dataDir := fmt.Sprintf("../../test/dataPartial/%s", p.DataSplit)

	// 自动检测数据分片文件
	splitID := fmt.Sprintf("train_split_%03d", p.ID)
	imagesPath := fmt.Sprintf("%s/%s_images.csv", dataDir, splitID)
	labelsPath := fmt.Sprintf("%s/%s_labels.csv", dataDir, splitID)

	fmt.Printf("参与方 %d 尝试载入数据集: %s\n", p.ID, imagesPath)

	// 检查文件是否存在
	if _, err := os.Stat(imagesPath); os.IsNotExist(err) {
		return fmt.Errorf("图像文件不存在: %s", imagesPath)
	}
	if _, err := os.Stat(labelsPath); os.IsNotExist(err) {
		return fmt.Errorf("标签文件不存在: %s", labelsPath)
	}

	// 载入图像数据
	images, err := p.loadImagesCSV(imagesPath)
	if err != nil {
		return fmt.Errorf("载入图像数据失败: %v", err)
	}

	// 载入标签数据
	labels, err := p.loadLabelsCSV(labelsPath)
	if err != nil {
		return fmt.Errorf("载入标签数据失败: %v", err)
	}

	p.Images = images
	p.Labels = labels

	fmt.Printf("参与方 %d 数据集载入完成: %d 个样本, %d 个特征\n",
		p.ID, len(images), len(images[0]))

	return nil
}

// loadImagesCSV 载入CSV格式的图像数据
func (p *Participant) loadImagesCSV(filepath string) ([][]float64, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	images := make([][]float64, len(records))
	for i, record := range records {
		images[i] = make([]float64, len(record))
		for j, val := range record {
			images[i][j], err = strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, err
			}
		}
	}

	return images, nil
}

// loadLabelsCSV 载入CSV格式的标签数据
func (p *Participant) loadLabelsCSV(filepath string) ([]int, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	labels := make([]int, len(records))
	for i, record := range records {
		labels[i], err = strconv.Atoi(record[0])
		if err != nil {
			return nil, err
		}
	}

	return labels, nil
}

// EncryptDataset 加密本地数据集
func (p *Participant) EncryptDataset() error {
	if !p.KeyManager.IsReady() {
		return fmt.Errorf("密钥未准备就绪，无法加密数据")
	}

	if len(p.Images) == 0 {
		return fmt.Errorf("数据集未载入，请先调用LoadDataset")
	}

	fmt.Printf("参与方 %d 开始加密数据集...\n", p.ID)

	// 获取加密所需的组件
	params := p.KeyManager.GetParams()
	pubKey := p.KeyManager.GetPublicKey()
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pubKey)

	// 加密每个样本
	encryptedImages := make([]*rlwe.Ciphertext, len(p.Images))
	for i, image := range p.Images {
		// 将图像数据转换为复数向量
		values := make([]complex128, len(image))
		for j, pixel := range image {
			// 归一化像素值到[0,1]范围
			values[j] = complex(pixel/255.0, 0)
		}

		// 编码
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(values, pt); err != nil {
			return fmt.Errorf("编码失败: %v", err)
		}

		// 加密
		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			return fmt.Errorf("加密失败: %v", err)
		}

		encryptedImages[i] = ct

		if (i+1)%100 == 0 {
			fmt.Printf("参与方 %d 已加密 %d/%d 个样本\n", p.ID, i+1, len(p.Images))
		}
	}

	fmt.Printf("参与方 %d 数据集加密完成: %d 个加密样本\n", p.ID, len(encryptedImages))

	return nil
}
