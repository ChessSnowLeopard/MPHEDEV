package main

import (
	"MPHEDev/pkg/core/coordinator/services"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	fmt.Print("请输入参与方数量: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	n, err := strconv.Atoi(line)
	if err != nil || n <= 0 {
		fmt.Println("输入有误，使用默认值3")
		n = 3
	}

	// 选择数据集划分方式
	fmt.Print("请选择数据集划分方式 (horizontal/vertical): ")
	splitLine, _ := reader.ReadString('\n')
	splitLine = strings.TrimSpace(splitLine)

	dataSplitType := "horizontal" // 默认值
	if splitLine == "horizontal" || splitLine == "vertical" {
		dataSplitType = splitLine
	} else {
		fmt.Println("输入有误，使用默认值horizontal")
	}

	coordinator, err := services.NewCoordinator(n, dataSplitType)
	if err != nil {
		panic(err)
	}

	fmt.Printf("协调器配置完成\n")
	fmt.Printf("预期参与方数量: %d\n", n)
	fmt.Printf("数据集划分方式: %s\n", dataSplitType)
	fmt.Printf("最小参与方阈值: %d (%.1f%%)\n", coordinator.GetMinParticipants(), float64(coordinator.GetMinParticipants())/float64(n)*100)
	fmt.Printf("本机IP地址: %s\n", coordinator.GetLocalIP())

	// 启动协调器
	if err := coordinator.Start(); err != nil {
		panic(err)
	}
}
