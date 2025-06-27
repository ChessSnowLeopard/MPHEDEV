package main

import (
	"MPHEDev/cmd/Coordinator/services"
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

	dataSplitType := "vertical" // 默认值
	if splitLine == "horizontal" || splitLine == "vertical" {
		dataSplitType = splitLine
	} else {
		fmt.Println("输入有误，使用默认值vertical")
	}

	coordinator, err := services.NewCoordinator(n, dataSplitType)
	if err != nil {
		panic(err)
	}

	fmt.Printf("协调器启动，监听端口 8080，等待 %d 个参与方连接...\n", n)
	fmt.Printf("数据集划分方式: %s\n", dataSplitType)
	fmt.Printf("最小参与方阈值: %d (%.1f%%)\n", coordinator.GetMinParticipants(), float64(coordinator.GetMinParticipants())/float64(n)*100)

	// 启动协调器
	if err := coordinator.Start(); err != nil {
		panic(err)
	}
}
