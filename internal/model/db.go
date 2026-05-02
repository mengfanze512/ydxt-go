package model

import (
	"fmt"
	"log"
	"os"
	"time"

	"yuedi_edu/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() {
	var err error

	dbConfig := config.GlobalConfig.Database
	dsn := dbConfig.DSN

	// [微信云托管] 自动读取环境变量组装 DSN
	// 只要在云托管控制台绑定了 MySQL 实例，云托管会自动向容器注入以下环境变量
	envMySQLAddr := os.Getenv("MYSQL_ADDRESS") // 格式: 10.x.x.x:3306
	envMySQLUser := os.Getenv("MYSQL_USERNAME")
	envMySQLPass := os.Getenv("MYSQL_PASSWORD")

	if envMySQLAddr != "" {
		// 云托管环境，覆盖本地配置
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/yuedi_edu?charset=utf8mb4&parseTime=True&loc=Local",
			envMySQLUser, envMySQLPass, envMySQLAddr)
		log.Println("Detected WeChat Cloud Hosting Environment. Using injected MYSQL_ADDRESS.")
	} else {
		log.Println("Running in local environment. Using DSN from config.yaml.")
	}

	// 连接 MySQL，默认打印 SQL 日志以便调试
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		DB = nil // 确保连接失败时 DB 为 nil
		log.Printf("Failed to connect database, but server will continue to start. Error: %v\n", err)
		return
	}

	// 获取底层 *sql.DB 配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Failed to get sql.DB: %v\n", err)
		return
	}

	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	// 设置了连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移数据库结构
	err = DB.AutoMigrate(&User{}, &Course{})
	if err != nil {
		log.Printf("Failed to auto migrate database: %v\n", err)
	}

	log.Println("Database connection established successfully!")
}
