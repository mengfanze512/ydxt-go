package model

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"ydxt-go/internal/config"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

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

	// 云托管环境默认不执行 AutoMigrate，避免对已建索引字段进行隐式改类型导致启动失败。
	// 如需启用迁移，请显式设置 ENABLE_AUTO_MIGRATE=true。
	enableAutoMigrate := os.Getenv("ENABLE_AUTO_MIGRATE") == "true"
	if enableAutoMigrate {
		err = DB.AutoMigrate(
			&User{}, &Course{}, &LiveRoom{}, &UserWallet{}, &LiveGiftRecord{},
			&ShopGoods{}, &CartItem{}, &Order{}, &OrderItem{}, &SheetMusic{},
			&UserCheckin{}, &PracticeRecord{}, &CommunityPost{}, &FinanceSettlement{},
		)
		if err != nil {
			log.Printf("Failed to auto migrate database: %v\n", err)
		}
	} else {
		log.Println("AutoMigrate skipped. Set ENABLE_AUTO_MIGRATE=true to enable schema migration.")
	}

	log.Println("Database connection established successfully!")
}

// InitRedis 初始化 Redis 连接
func InitRedis() {
	rdbConfig := config.GlobalConfig.Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:     rdbConfig.Addr,
		Password: rdbConfig.Password,
		DB:       rdbConfig.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		log.Printf("Failed to connect Redis: %v\n", err)
	} else {
		log.Println("Redis connection established successfully!")
	}
}
