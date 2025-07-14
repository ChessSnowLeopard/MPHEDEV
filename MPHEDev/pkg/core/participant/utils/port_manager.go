package utils

import (
	"fmt"
	"net"
	"strconv"
)

// PortManager 端口管理器
type PortManager struct {
	preferredPort int
	portRange     [2]int // [min, max]
	currentPort   int
}

// NewPortManager 创建端口管理器
func NewPortManager(preferredPort int, portRange [2]int) *PortManager {
	return &PortManager{
		preferredPort: preferredPort,
		portRange:     portRange,
		currentPort:   0,
	}
}

// FindAvailablePort 寻找可用端口
func (pm *PortManager) FindAvailablePort() (int, error) {
	fmt.Printf("开始寻找可用端口，首选端口: %d，端口范围: [%d-%d]\n", pm.preferredPort, pm.portRange[0], pm.portRange[1])

	// 首先尝试首选端口
	if pm.IsPortAvailable(pm.preferredPort) {
		pm.currentPort = pm.preferredPort
		fmt.Printf("✓ 使用首选端口: %d\n", pm.currentPort)
		return pm.currentPort, nil
	}

	fmt.Printf("⚠️ 首选端口 %d 被占用，开始寻找其他可用端口...\n", pm.preferredPort)

	// 在端口范围内寻找可用端口
	for port := pm.portRange[0]; port <= pm.portRange[1]; port++ {
		if port == pm.preferredPort {
			continue // 跳过首选端口，已经检查过了
		}

		if pm.IsPortAvailable(port) {
			pm.currentPort = port
			fmt.Printf("✓ 找到可用端口: %d (首选端口 %d 被占用)\n", pm.currentPort, pm.preferredPort)
			return pm.currentPort, nil
		} else {
			fmt.Printf("  - 端口 %d 被占用\n", port)
		}
	}

	return 0, fmt.Errorf("在端口范围 [%d-%d] 内未找到可用端口", pm.portRange[0], pm.portRange[1])
}

// IsPortAvailable 检查端口是否可用
func (pm *PortManager) IsPortAvailable(port int) bool {
	// 尝试监听端口
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

// GetCurrentPort 获取当前使用的端口
func (pm *PortManager) GetCurrentPort() int {
	return pm.currentPort
}

// GetPreferredPort 获取首选端口
func (pm *PortManager) GetPreferredPort() int {
	return pm.preferredPort
}

// GetPortRange 获取端口范围
func (pm *PortManager) GetPortRange() [2]int {
	return pm.portRange
}
