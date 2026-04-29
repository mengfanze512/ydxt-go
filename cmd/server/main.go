package main

import (
	"fmt"
	"log"

	"yuedi_edu/internal/api"
	"yuedi_edu/internal/config"
	"yuedi_edu/internal/model"
)

func main() {
	// 1. 加载本地配置文件 config.yaml
	config.InitConfig()

	// 2. 初始化数据库 (GORM + MySQL)
	model.InitDB()

	// 3. 注册所有的 Gin 路由
	router := api.InitRouter()

	// 4. 启动服务 (默认监听 8080 端口，微信云托管默认也是探测 80 端口)
	port := fmt.Sprintf(":%d", config.GlobalConfig.Server.Port)
	log.Printf("Starting Server on port %s...\n", port)
	
	if err := router.Run(port); err != nil {
		log.Fatalf("Server started failed: %v", err)
	}
}
