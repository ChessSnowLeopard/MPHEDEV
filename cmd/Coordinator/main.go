package main

import (
	"MPHEDev/pkg/core/coordinator/services"
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var globalDataSplitType string

func main() {
	// 1. 启动时命令行选择数据集划分方式
	fmt.Print("请选择数据集划分方式 (horizontal/vertical): ")
	reader := bufio.NewReader(os.Stdin)
	splitLine, _ := reader.ReadString('\n')
	splitLine = strings.TrimSpace(splitLine)
	globalDataSplitType = "horizontal" // 默认值
	if splitLine == "horizontal" || splitLine == "vertical" {
		globalDataSplitType = splitLine
	} else {
		fmt.Println("输入有误，使用默认值horizontal")
	}

	r := gin.Default()
	// 允许跨域
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 包装InitHandler，将命令行选定的数据集划分方式注入
	r.POST("/init", func(ctx *gin.Context) {
		ctx.Set("data_split_type", globalDataSplitType)
		services.InitHandler(ctx)
	})

	protected := r.Group("/", services.RequireCoordinator())
	{
		protected.POST("/keys/public", services.PostPublicKeyHandler)
		protected.POST("/keys/secret", services.PostSecretKeyHandler)
		protected.GET("/setup/status", services.GetSetupStatusHandler)
		// ...其他依赖 Coordinator 的接口...
	}

	// 其他路由注册...
	r.SetTrustedProxies([]string{"127.0.0.1"})
	r.Run(":8080")
}
