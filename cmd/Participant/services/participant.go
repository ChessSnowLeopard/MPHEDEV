package services

import (
	"MPHEDev/cmd/Participant/coordinator"
	"MPHEDev/cmd/Participant/crypto"
	"MPHEDev/cmd/Participant/network"
	"MPHEDev/cmd/Participant/server"
	"MPHEDev/cmd/Participant/types"
	"fmt"
	"net/http"
	"time"
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
