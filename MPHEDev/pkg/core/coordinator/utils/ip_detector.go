package utils

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

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
