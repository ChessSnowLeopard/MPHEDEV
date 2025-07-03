package network

import (
	"MPHEDev/pkg/core/participant/types"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// PeerManager P2P网络管理
type PeerManager struct {
	Peers map[int]string // ID -> URL映射
	mu    sync.RWMutex
}

// NewPeerManager 创建新的P2P网络管理器
func NewPeerManager() *PeerManager {
	return &PeerManager{
		Peers: make(map[int]string),
	}
}

// AddPeer 添加对等节点
func (pm *PeerManager) AddPeer(peerID int, peerURL string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.Peers[peerID] = peerURL
	fmt.Printf("添加对等节点: %d (%s)\n", peerID, peerURL)
}

// GetPeerURL 获取对等节点URL
func (pm *PeerManager) GetPeerURL(peerID int) (string, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	url, exists := pm.Peers[peerID]
	return url, exists
}

// GetPeers 获取所有对等节点
func (pm *PeerManager) GetPeers() map[int]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[int]string)
	for id, url := range pm.Peers {
		result[id] = url
	}
	return result
}

// DiscoverPeers 从协调器发现其他参与方
func (pm *PeerManager) DiscoverPeers(coordinatorURL string, client *types.HTTPClient, selfID int) error {
	resp, err := client.Client.Get(coordinatorURL + "/participants/list")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var peers []types.PeerInfo
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return err
	}

	// 添加其他参与方到Peers映射
	for _, peer := range peers {
		if peer.ID != selfID {
			pm.AddPeer(peer.ID, peer.URL)
		}
	}

	fmt.Printf("发现 %d 个其他参与方\n", len(pm.Peers))
	return nil
}

// GetLocalIP 获取本机IP地址
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("没有找到有效的IP地址")
}

// ReportURL 向协调器上报自己的URL
func (pm *PeerManager) ReportURL(coordinatorURL string, client *types.HTTPClient, selfID int, port int) error {
	// 获取本机IP地址
	localIP, err := GetLocalIP()
	if err != nil {
		return fmt.Errorf("获取本机IP失败: %v", err)
	}

	myURL := fmt.Sprintf("http://%s:%d", localIP, port)

	reqBody, _ := json.Marshal(types.PeerInfo{
		ID:  selfID,
		URL: myURL,
	})

	resp, err := client.Client.Post(coordinatorURL+"/participants/url", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("上报URL: %s\n", myURL)
	return nil
}
