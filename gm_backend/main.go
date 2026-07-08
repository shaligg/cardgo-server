package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/bigfish/gm_backend/router"
)

func main() {
	// 创建路由
	r := gin.Default()

	// 配置路由
	router.SetupRoutes(r)

	// 启动服务器
	port := 8080
	fmt.Printf("GM后台服务启动成功，监听端口: %d\n", port)
	r.Run(fmt.Sprintf(":%d", port))
}