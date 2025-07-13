package services

import (
	"MPHEDev/pkg/core/participant/coordinator"
	"MPHEDev/pkg/core/participant/network"
	"MPHEDev/pkg/core/participant/server"
	"MPHEDev/pkg/core/participant/utils"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

// RegistryService 注册服务
type RegistryService struct {
	participant *Participant
}

// NewRegistryService 创建新的注册服务
func NewRegistryService(participant *Participant) *RegistryService {
	return &RegistryService{
		participant: participant,
	}
}

// Register 注册到协调器并启动P2P服务器
func (rs *RegistryService) Register(coordinatorURL string) error {
	// 1. 自动检测数据集划分方式
	dataSplit := rs.autoDetectDataSplit()
	if dataSplit == "" {
		return fmt.Errorf("未找到本地分片文件，无法注册")
	}
	rs.participant.DataManager.SetDataSplit(dataSplit)
	shardID := rs.getLocalShardID(dataSplit)
	if shardID == "" {
		return fmt.Errorf("未找到本地分片文件，无法注册")
	}
	// 2. 创建协调器客户端
	rs.participant.CoordinatorClient = coordinator.NewCoordinatorClient(coordinatorURL, rs.participant.Client)
	// 3. 注册获取ID
	regResp, err := rs.participant.CoordinatorClient.Register(shardID)
	if err != nil {
		return fmt.Errorf("注册失败: %v", err)
	}
	rs.participant.ID = regResp.ParticipantID

	// 4. 动态分配端口
	portManager := utils.NewPortManager(8081, [2]int{8081, 8090}) // 首选8081，范围8081-8090
	port, err := portManager.FindAvailablePort()
	if err != nil {
		return fmt.Errorf("端口分配失败: %v", err)
	}
	rs.participant.Port = port
	fmt.Printf("参与方 %d 使用端口: %d\n", rs.participant.ID, rs.participant.Port)

	// 5. 创建P2P网络管理器
	rs.participant.PeerManager = network.NewPeerManager()

	// 6. 创建心跳管理器
	rs.participant.HeartbeatManager = network.NewHeartbeatManager(coordinatorURL, rs.participant.Client, rs.participant.ID)

	// 7. 启动P2P服务器
	if err := rs.startHTTPServer(); err != nil {
		return fmt.Errorf("启动P2P服务器失败: %v", err)
	}

	// 8. 等待服务器启动
	time.Sleep(1 * time.Second)

	// 9. 向协调器上报自己的URL
	if err := rs.participant.PeerManager.ReportURL(coordinatorURL, rs.participant.Client, rs.participant.ID, rs.participant.Port); err != nil {
		return fmt.Errorf("上报URL失败: %v", err)
	}

	// 10. 获取其他参与方的URL
	if err := rs.participant.PeerManager.DiscoverPeers(coordinatorURL, rs.participant.Client, rs.participant.ID); err != nil {
		return fmt.Errorf("发现其他参与方失败: %v", err)
	}

	// 11. 启动心跳机制
	rs.participant.HeartbeatManager.Start()

	// 12. 启动在线状态监控
	rs.participant.HeartbeatManager.StartOnlineStatusMonitor()

	// 13. 发送初始心跳，确保自己能被识别为在线
	if err := rs.participant.HeartbeatManager.SendInitialHeartbeat(); err != nil {
		return fmt.Errorf("发送初始心跳失败: %v", err)
	}

	// 14. 获取参数并设置数据集划分方式
	paramsResp, err := rs.participant.CoordinatorClient.GetParams()
	if err != nil {
		return fmt.Errorf("获取参数失败: %v", err)
	}

	// 设置数据集划分方式
	rs.participant.DataManager.SetDataSplit(paramsResp.DataSplitType)

	// 15. 获取在线成员列表
	if err := rs.UpdateOnlineParticipants(); err != nil {
		return fmt.Errorf("获取在线成员列表失败: %v", err)
	}

	// 16. 获取参与方数量信息（新增）
	if err := rs.updateParticipantCount(); err != nil {
		return fmt.Errorf("获取参与方数量失败: %v", err)
	}

	return nil
}

// Unregister 注销参与方
func (rs *RegistryService) Unregister() error {
	shardID := rs.getLocalShardID(rs.participant.DataManager.GetDataSplit())
	if shardID == "" {
		return fmt.Errorf("未找到本地分片文件，无法注销")
	}
	return rs.participant.CoordinatorClient.Unregister(shardID)
}

// UpdateOnlineParticipants 更新在线参与方列表
func (rs *RegistryService) UpdateOnlineParticipants() error {
	// 从协调器获取在线参与方列表
	onlineParticipants := rs.participant.HeartbeatManager.GetOnlinePeers()

	// 清空之前的参与方列表
	rs.participant.PeerManager.ClearPeers()

	// 更新P2P网络管理器中的参与方列表
	for id, url := range onlineParticipants {
		if id != rs.participant.ID { // 不添加自己
			rs.participant.PeerManager.AddPeer(id, url)
		}
	}

	// 统计在线参与方数量（onlineParticipants已经包含所有在线参与方）
	totalOnline := len(onlineParticipants)
	fmt.Printf("当前在线参与方: %d 个 (包括自己)\n", totalOnline)
	fmt.Printf("  参与方 %d: %s (自己)\n", rs.participant.ID, rs.participant.HTTPServer.GetLocalIP())
	for id, url := range onlineParticipants {
		if id != rs.participant.ID { // 只显示其他参与方
			fmt.Printf("  参与方 %d: %s\n", id, url)
		}
	}

	return nil
}

// updateParticipantCount 更新参与方数量信息
func (rs *RegistryService) updateParticipantCount() error {
	// 获取协调器详细状态
	_, err := rs.participant.CoordinatorClient.GetDetailedStatus()
	if err != nil {
		return fmt.Errorf("获取协调器详细状态失败: %v", err)
	}

	fmt.Printf("参与方数量信息更新完成\n")
	return nil
}

// startHTTPServer 启动HTTP服务器
func (rs *RegistryService) startHTTPServer() error {
	// 创建HTTP处理器
	handlers := server.NewHandlers(rs.participant.KeyManager, rs.participant.DecryptionService, rs.participant.RefreshService)
	handlerMap := handlers.GetHandlers()

	// 创建HTTP服务器
	rs.participant.HTTPServer = server.NewHTTPServer(rs.participant.Port, handlerMap, rs.participant)

	// 启动服务器
	return rs.participant.HTTPServer.Start()
}

// getLocalShardID 获取本地分片ID
func (rs *RegistryService) getLocalShardID(dataSplit string) string {
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
func (rs *RegistryService) autoDetectDataSplit() string {
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
