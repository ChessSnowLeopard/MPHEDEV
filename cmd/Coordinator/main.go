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
	// 注册状态查询接口
	router.GET("/api/coordinator/status", services.RequireCoordinator(), services.GetCoordinatorStatusHandler)
	// 注册密钥进度查询接口
	router.GET("/api/coordinator/key-progress", services.RequireCoordinator(), services.GetKeyProgressHandler)

	port := "8060"
	fmt.Printf("Coordinator HTTP server running on port %s\n", port)
	if err := router.Run(":" + port); err != nil {
		panic(err)
	}
	//启动在coordinator_handlers.go中的http服务init后
}
