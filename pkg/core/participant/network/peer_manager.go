package network

import (
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strings"
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

// ClearPeers 清空所有对等节点
func (pm *PeerManager) ClearPeers() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.Peers = make(map[int]string)
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

// GetLocalIP 获取本机IP地址，优先使用Radmin VPN接口
func GetLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// 优先查找Radmin VPN接口
	for _, iface := range interfaces {
		if strings.Contains(strings.ToLower(iface.Name), "radmin") {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						// 返回Radmin VPN接口的IP地址
						return ipnet.IP.String(), nil
					}
				}
			}
		}
	}

	// 优先选择的接口名称（按优先级排序）
	preferredInterfaces := []string{"wlan", "wifi", "wireless", "ethernet", "eth", "en"}

	// 首先尝试找到优先接口
	for _, preferred := range preferredInterfaces {
		for _, iface := range interfaces {
			if strings.Contains(strings.ToLower(iface.Name), preferred) {
				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}

				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							// 检查是否是私有IP地址
							if isPrivateIP(ipnet.IP) {
								return ipnet.IP.String(), nil
							}
						}
					}
				}
			}
		}
	}

	// 如果没找到优先接口，遍历所有接口
	for _, iface := range interfaces {
		// 跳过回环接口和虚拟接口
		if iface.Flags&net.FlagLoopback != 0 ||
			strings.Contains(strings.ToLower(iface.Name), "vmware") ||
			strings.Contains(strings.ToLower(iface.Name), "virtual") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					// 优先选择私有IP地址
					if isPrivateIP(ipnet.IP) {
						return ipnet.IP.String(), nil
					}
				}
			}
		}
	}

	// 最后尝试所有IPv4地址
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

	return "", fmt.Errorf("未找到有效的IP地址")
}

// isPrivateIP 检查是否是私有IP地址
func isPrivateIP(ip net.IP) bool {
	// 私有IP地址范围
	privateRanges := []struct {
		start net.IP
		end   net.IP
	}{
		{net.ParseIP("10.0.0.0"), net.ParseIP("10.255.255.255")},     // 10.0.0.0/8
		{net.ParseIP("172.16.0.0"), net.ParseIP("172.31.255.255")},   // 172.16.0.0/12
		{net.ParseIP("192.168.0.0"), net.ParseIP("192.168.255.255")}, // 192.168.0.0/16
	}

	for _, r := range privateRanges {
		if bytes.Compare(ip, r.start) >= 0 && bytes.Compare(ip, r.end) <= 0 {
			return true
		}
	}
	return false
}

// ReportURL 向协调器上报自己的URL
func (pm *PeerManager) ReportURL(coordinatorURL string, client *types.HTTPClient, selfID int, port int) error {
	// 使用正确的IP获取函数，优先查找Radmin VPN接口
	localIP, err := utils.GetLocalIP()
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
