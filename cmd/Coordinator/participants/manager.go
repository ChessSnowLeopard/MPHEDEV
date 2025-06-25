package participants

import (
	"MPHEDev/cmd/Coordinator/utils"
	"fmt"
	"sync"
	"time"
)

// Manager 参与者管理器
type Manager struct {
	participants    map[int]*utils.ParticipantInfo
	participantURLs map[int]string // 参与方ID -> URL映射
	nextID          int
	mu              sync.RWMutex

	// 在线状态管理
	heartbeats        map[int]time.Time // 参与方ID -> 最后心跳时间
	onlineTimeout     time.Duration     // 心跳超时时间
	minParticipants   int               // 最小参与方数量阈值
	heartbeatInterval time.Duration     // 心跳间隔
}

// NewManager 创建新的参与者管理器
func NewManager(expectedN int) *Manager {
	// 计算最小参与方数量：至少需要2个参与方，或者期望数量的2/3
	minParticipants := expectedN

	return &Manager{
		participants:      make(map[int]*utils.ParticipantInfo),
		participantURLs:   make(map[int]string),
		nextID:            1,
		heartbeats:        make(map[int]time.Time),
		onlineTimeout:     10 * time.Second,
		minParticipants:   minParticipants,
		heartbeatInterval: 5 * time.Second,
	}
}

// RegisterParticipant 注册新参与方
func (m *Manager) RegisterParticipant() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	m.participants[id] = &utils.ParticipantInfo{ID: id, Status: "registered"}
	return id
}

// AddParticipantURL 添加参与方URL
func (m *Manager) AddParticipantURL(participantID int, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.participants[participantID]; !exists {
		return fmt.Errorf("参与方 %d 不存在", participantID)
	}

	m.participantURLs[participantID] = url
	fmt.Printf("添加参与方 %d URL: %s\n", participantID, url)
	return nil
}

// GetParticipantURL 获取参与方URL
func (m *Manager) GetParticipantURL(participantID int) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, exists := m.participantURLs[participantID]
	return url, exists
}

// GetAllParticipantURLs 获取所有参与方URL列表
func (m *Manager) GetAllParticipantURLs() []utils.PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var peerInfos []utils.PeerInfo
	for id, url := range m.participantURLs {
		peerInfos = append(peerInfos, utils.PeerInfo{
			ID:  id,
			URL: url,
		})
	}

	return peerInfos
}

// GetParticipants 获取所有参与方信息
func (m *Manager) GetParticipants() []*utils.ParticipantInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var participants []*utils.ParticipantInfo
	for _, p := range m.participants {
		participants = append(participants, p)
	}
	return participants
}

// UpdateHeartbeat 更新参与方心跳
func (m *Manager) UpdateHeartbeat(participantID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.participants[participantID]; !exists {
		return fmt.Errorf("参与方 %d 不存在", participantID)
	}

	m.heartbeats[participantID] = time.Now()
	return nil
}

// GetOnlineParticipants 获取在线参与方列表
func (m *Manager) GetOnlineParticipants() []utils.PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var onlineParticipants []utils.PeerInfo
	now := time.Now()

	for id, url := range m.participantURLs {
		if lastHeartbeat, exists := m.heartbeats[id]; exists {
			if now.Sub(lastHeartbeat) <= m.onlineTimeout {
				onlineParticipants = append(onlineParticipants, utils.PeerInfo{
					ID:  id,
					URL: url,
				})
			}
		}
	}

	return onlineParticipants
}

// GetOnlineStatus 获取在线状态信息
func (m *Manager) GetOnlineStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	onlineCount := 0
	totalCount := len(m.participants)

	for _, lastHeartbeat := range m.heartbeats {
		if now.Sub(lastHeartbeat) <= m.onlineTimeout {
			onlineCount++
		}
	}

	onlinePercentage := float64(onlineCount) / float64(totalCount) * 100
	canProceed := onlineCount >= m.minParticipants

	return map[string]interface{}{
		"online_count":       onlineCount,
		"total_count":        totalCount,
		"online_percentage":  onlinePercentage,
		"min_participants":   m.minParticipants,
		"can_proceed":        canProceed,
		"online_timeout":     m.onlineTimeout.Seconds(),
		"heartbeat_interval": m.heartbeatInterval.Seconds(),
	}
}

// CleanupOfflineParticipants 清理离线参与方
func (m *Manager) CleanupOfflineParticipants() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, lastHeartbeat := range m.heartbeats {
		if now.Sub(lastHeartbeat) > m.onlineTimeout {
			delete(m.heartbeats, id)
			fmt.Printf("清理离线参与方: %d\n", id)
		}
	}
}

// StartHeartbeatCleanup 启动心跳清理协程
func (m *Manager) StartHeartbeatCleanup() {
	go func() {
		ticker := time.NewTicker(m.heartbeatInterval)
		defer ticker.Stop()

		for range ticker.C {
			m.CleanupOfflineParticipants()
		}
	}()
}

// GetMinParticipants 获取最小参与方数量
func (m *Manager) GetMinParticipants() int {
	return m.minParticipants
}

// GetExpectedN 获取期望的参与方数量
func (m *Manager) GetExpectedN() int {
	return m.minParticipants
}
