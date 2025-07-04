package main

import (
	"MPHEDev/pkg/core/coordinator/services"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	// 注册初始化协调器的接口
	router.POST("/api/coordinator/init", services.InitHandler)

	port := "8080"
	fmt.Printf("Coordinator HTTP server running on port %s\n", port)
	if err := router.Run(":" + port); err != nil {
		panic(err)
	}
	//启动在coordinator_handlers.go中的http服务init后
}
