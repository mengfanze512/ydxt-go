package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Wechat   WechatConfig
	Agora    AgoraConfig
}

type ServerConfig struct {
	Port int
	Mode string
}

type DatabaseConfig struct {
	DSN          string
	MaxIdleConns int
	MaxOpenConns int
}

type JWTConfig struct {
	Secret      string
	ExpireHours int `mapstructure:"expire_hours"`
}

type WechatConfig struct {
	AppID       string `mapstructure:"app_id"`
	AppSecret   string `mapstructure:"app_secret"`
	MchID       string `mapstructure:"mch_id"`
	MchAPIV3Key string `mapstructure:"mch_api_v3_key"`
}

type AgoraConfig struct {
	AppID          string `mapstructure:"app_id"`
	AppCertificate string `mapstructure:"app_certificate"`
}

var GlobalConfig *Config

func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config") // 寻找 config.yaml 的路径
	viper.AddConfigPath("../config")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
	}

	GlobalConfig = &Config{}
	if err := viper.Unmarshal(GlobalConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v\n", err)
	}

	log.Println("Config loaded successfully!")
}
