package main

import (
	"fmt"
	"os"
	"os/exec"
)

// callCtCNN 调用ctCNN EXE
func main() {
	fmt.Printf("🔄 开始调用ctCNN EXE...\n")

	// 构建ctCNN EXE的路径 - 修正相对路径
	exePath := "../ctCNN/ctCNN.exe"

	// 创建命令
	cmd := exec.Command(exePath)

	// 设置工作目录
	cmd.Dir = "."

	// 直接连接到标准输出和错误输出，实现实时显示
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行命令
	fmt.Printf("执行命令: %s (工作目录: %s)\n", exePath, cmd.Dir)
	err := cmd.Run()

	if err != nil {
		fmt.Printf("❌ ctCNN执行失败: %v\n", err)
	} else {
		fmt.Printf("✅ ctCNN执行成功\n")
	}
}
