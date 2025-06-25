package main

import (
	"MPHEDev/cmd/Participant/services"
	"fmt"
	"sync"
	"time"
)

func main() {
	fmt.Println("=== 动态在线状态管理系统测试 ===")
	fmt.Println("此测试将启动多个参与方，验证静默模式和在线状态检查功能")
	fmt.Println()

	// 启动协调器（需要手动启动）
	fmt.Println("请先启动协调器 (Coordinator.exe)，然后按回车继续...")
	fmt.Scanln()

	// 创建多个参与方
	participants := make([]*services.Participant, 2)
	var wg sync.WaitGroup

	// 启动参与方
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			participant := services.NewParticipant()
			participants[index] = participant

			fmt.Printf("启动参与方 %d...\n", index+1)

			// 注册到协调器
			if err := participant.Register("http://localhost:8080"); err != nil {
				fmt.Printf("参与方 %d 注册失败: %v\n", index+1, err)
				return
			}

			fmt.Printf("参与方 %d 注册成功，ID: %d\n", index+1, participant.ID)

			// 等待密钥分发完成
			<-participant.ReadyCh
			fmt.Printf("参与方 %d 密钥分发完成\n", index+1)

			// 等待一段时间让心跳机制稳定
			time.Sleep(3 * time.Second)

			// 显示在线状态
			fmt.Printf("\n参与方 %d 查看在线状态:\n", index+1)
			if err := participant.ShowOnlineStatus(); err != nil {
				fmt.Printf("参与方 %d 获取在线状态失败: %v\n", index+1, err)
			}

			// 尝试发起协作解密
			fmt.Printf("\n参与方 %d 尝试发起协作解密...\n", index+1)
			if err := participant.RequestCollaborativeDecrypt(); err != nil {
				fmt.Printf("参与方 %d 协作解密失败: %v\n", index+1, err)
			} else {
				fmt.Printf("参与方 %d 协作解密成功！\n", index+1)
			}

			// 保持运行一段时间
			time.Sleep(5 * time.Second)
		}(i)

		// 间隔启动，模拟不同时间加入
		time.Sleep(1 * time.Second)
	}

	// 等待所有参与方完成
	wg.Wait()

	fmt.Println("\n=== 测试完成 ===")
	fmt.Println("所有参与方已退出")
	fmt.Println("\n注意：在菜单模式下，应该不会看到自动的状态更新输出")
}
