package network

import (
	"MPHEDev/pkg/core/participant/types"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HeartbeatManager 心跳和在线状态管理
type HeartbeatManager struct {
	coordinatorURL     string
	heartbeatTicker    *time.Ticker
	heartbeatStopCh    chan struct{}
	onlinePeers        map[int]string
	lastPeerUpdate     time.Time
	peerUpdateInterval time.Duration
	silentMode         bool
	client             *types.HTTPClient
	participantID      int
	interval           time.Duration
	stopCh             chan struct{}
}

// NewHeartbeatManager 创建新的心跳管理器
func NewHeartbeatManager(coordinatorURL string, client *types.HTTPClient, participantID int) *HeartbeatManager {
	return &HeartbeatManager{
		coordinatorURL:     coordinatorURL,
		heartbeatStopCh:    make(chan struct{}),
		onlinePeers:        make(map[int]string),
		peerUpdateInterval: 10 * time.Second,
		silentMode:         false,
		client:             client,
		participantID:      participantID,
		interval:           5 * time.Second, // 5秒心跳间隔
		stopCh:             make(chan struct{}),
	}
}

// Start 启动心跳管理器
func (hm *HeartbeatManager) Start() {
	fmt.Printf("心跳管理器启动，参与方ID: %d\n", hm.participantID)
	fmt.Printf("心跳间隔: %v\n", hm.interval)
	fmt.Printf("协调器URL: %s\n", hm.coordinatorURL)

	go hm.run()
}

// run 运行心跳循环
func (hm *HeartbeatManager) run() {
	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := hm.sendHeartbeat(); err != nil {
				fmt.Printf("心跳发送失败: %v\n", err)
			}
		case <-hm.stopCh:
			fmt.Printf("心跳管理器停止\n")
			return
		}
	}
}

// sendHeartbeat 发送心跳
func (hm *HeartbeatManager) sendHeartbeat() error {
	reqBody := map[string]interface{}{
		"participant_id": hm.participantID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(hm.coordinatorURL+"/heartbeat", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("心跳响应状态码错误: %d", resp.StatusCode)
	}

	return nil
}

// StartOnlineStatusMonitor 启动在线状态监控
func (hm *HeartbeatManager) StartOnlineStatusMonitor() {
	go func() {
		ticker := time.NewTicker(hm.peerUpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hm.updateOnlinePeers()
			case <-hm.heartbeatStopCh:
				return
			}
		}
	}()

	fmt.Printf("启动在线状态监控\n")
}

// updateOnlinePeers 更新在线参与方列表
func (hm *HeartbeatManager) updateOnlinePeers() {
	resp, err := hm.client.Client.Get(hm.coordinatorURL + "/participants/online")
	if err != nil {
		if !hm.silentMode {
			fmt.Printf("获取在线列表失败: %v\n", err)
		}
		return
	}
	defer resp.Body.Close()

	var onlinePeers []types.PeerInfo
	if err := json.NewDecoder(resp.Body).Decode(&onlinePeers); err != nil {
		if !hm.silentMode {
			fmt.Printf("解析在线列表失败: %v\n", err)
		}
		return
	}

	hm.onlinePeers = make(map[int]string)
	for _, peer := range onlinePeers {
		hm.onlinePeers[peer.ID] = peer.URL
	}
	hm.lastPeerUpdate = time.Now()

	if !hm.silentMode {
		fmt.Printf("更新在线列表: %d 个参与方在线\n", len(hm.onlinePeers))
	}
}

// GetOnlinePeers 获取在线参与方列表
func (hm *HeartbeatManager) GetOnlinePeers() map[int]string {
	result := make(map[int]string)
	for id, url := range hm.onlinePeers {
		result[id] = url
	}
	return result
}

// GetOnlineStatus 获取在线状态信息
func (hm *HeartbeatManager) GetOnlineStatus() (map[string]interface{}, error) {
	resp, err := hm.client.Client.Get(hm.coordinatorURL + "/status/online")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return status, nil
}

// SetSilentMode 设置静默模式
func (hm *HeartbeatManager) SetSilentMode(silent bool) {
	hm.silentMode = silent
}

// StopHeartbeat 停止心跳机制
func (hm *HeartbeatManager) StopHeartbeat() {
	if hm.heartbeatTicker != nil {
		close(hm.heartbeatStopCh)
	}
}

// ShowOnlineStatus 显示在线状态信息
func (hm *HeartbeatManager) ShowOnlineStatus() error {
	status, err := hm.GetOnlineStatus()
	if err != nil {
		return err
	}

	fmt.Println("\n=== 在线状态信息 ===")
	fmt.Printf("在线参与方数量: %v\n", status["online_count"])
	fmt.Printf("总参与方数量: %v\n", status["total_count"])
	fmt.Printf("最小阈值: %v\n", status["min_participants"])
	fmt.Printf("可以协作: %v\n", status["can_proceed"])
	fmt.Printf("心跳超时: %v 秒\n", status["online_timeout"])
	fmt.Printf("心跳间隔: %v 秒\n", status["heartbeat_interval"])

	// 显示在线参与方列表
	onlinePeers := hm.GetOnlinePeers()
	fmt.Printf("\n在线参与方列表 (%d 个):\n", len(onlinePeers))
	for id, url := range onlinePeers {
		fmt.Printf("   参与方 %d: %s\n", id, url)
	}

	return nil
}

// CheckOnlineStatusBeforeOperation 在操作前检查在线状态
func (hm *HeartbeatManager) CheckOnlineStatusBeforeOperation() error {
	status, err := hm.GetOnlineStatus()
	if err != nil {
		return fmt.Errorf("获取在线状态失败: %v", err)
	}

	// 检查字段是否存在
	onlineCountVal, ok := status["online_count"]
	if !ok || onlineCountVal == nil {
		return fmt.Errorf("在线状态信息不完整: online_count 字段缺失")
	}
	onlineCount := int(onlineCountVal.(float64))

	totalCountVal, ok := status["total_count"]
	if !ok || totalCountVal == nil {
		return fmt.Errorf("在线状态信息不完整: total_count 字段缺失")
	}
	totalCount := int(totalCountVal.(float64))

	minParticipantsVal, ok := status["min_participants"]
	if !ok || minParticipantsVal == nil {
		return fmt.Errorf("在线状态信息不完整: min_participants 字段缺失")
	}
	minParticipants := int(minParticipantsVal.(float64))

	canCollaborateVal, ok := status["can_proceed"]
	if !ok || canCollaborateVal == nil {
		return fmt.Errorf("在线状态信息不完整: can_proceed 字段缺失")
	}
	canCollaborate := canCollaborateVal.(bool)

	fmt.Printf(" 检查在线状态: %d/%d 个参与方在线，最小阈值: %d\n", onlineCount, totalCount, minParticipants)

	if !canCollaborate {
		return fmt.Errorf(" 在线参与方数量不足，无法进行协作操作 (需要至少 %d 个参与方在线，当前 %d 个)", minParticipants, onlineCount)
	}

	return nil
}

// SendInitialHeartbeat 发送初始心跳
func (hm *HeartbeatManager) SendInitialHeartbeat() error {
	return hm.sendHeartbeat()
}
