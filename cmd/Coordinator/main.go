package main

import (
	"MPHEDev/cmd/Coordinator/handlers"
	"MPHEDev/cmd/Coordinator/services"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

	coordinator, err := services.NewCoordinator(n)
	if err != nil {
		panic(err)
	}

	r := gin.Default()

	// 注册路由处理器
	r.POST("/register", handlers.RegisterHandler(coordinator))
	r.GET("/params/ckks", handlers.GetCKKSParamsHandler(coordinator))
	r.POST("/keys/public", handlers.PostPublicKeyHandler(coordinator))
	r.POST("/keys/secret", handlers.PostSecretKeyHandler(coordinator))
	r.POST("/keys/galois", handlers.PostGaloisKeyHandler(coordinator))
	r.POST("/keys/relin", handlers.PostRelinearizationKeyHandler(coordinator))
	r.GET("/keys/relin/round1", handlers.GetRelinearizationKeyRound1AggregatedHandler(coordinator))
	r.GET("/participants", handlers.GetParticipantsHandler(coordinator))
	r.GET("/setup/status", handlers.GetSetupStatusHandler(coordinator))

	fmt.Printf("协调器启动，监听端口 8080，等待 %d 个参与方连接...\n", n)
	r.Run(":8080")
}
