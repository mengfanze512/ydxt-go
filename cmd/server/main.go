package main

import (
	"fmt"
	"log"
	"os"

	"ydxt-go/internal/api"
	"ydxt-go/internal/config"
	"ydxt-go/internal/model"
)

func main() {
	// 1. 加载本地配置文件 config.yaml
	config.InitConfig()

	// 2. 初始化数据库 (GORM + MySQL)
	model.InitDB()
	model.InitRedis()

	// 3. 注册所有的 Gin 路由
	router := api.InitRouter()

	// 4. 启动服务
	// 优先使用云环境注入的 PORT，避免健康检查端口与服务监听端口不一致。
	port := os.Getenv("PORT")
	if port == "" {
		port = fmt.Sprintf("%d", config.GlobalConfig.Server.Port)
	}
	port = ":" + port
	log.Printf("Starting Server on port %s...\n", port)
	
	if err := router.Run(port); err != nil {
		log.Fatalf("Server started failed: %v", err)
	}
}
