package config

import "os"

// Config 用于集中管理后端运行所需的配置项（通过环境变量注入）。
type Config struct {
	// ServerAddr 为 Gin 监听地址，例如 :8888
	ServerAddr string
	// MySQLDSN 为 MySQL 连接串，例如 user:pass@tcp(127.0.0.1:3306)/chat_room?parseTime=true
	MySQLDSN string
	// RedisAddr 为 Redis 地址，例如 127.0.0.1:6379
	RedisAddr string
	// RedisPassword 为 Redis 密码（无密码可为空）
	RedisPassword string
	// RedisDB 为 Redis DB 索引（字符串形式，便于从 env 读取）
	RedisDB string
	// JWTSecret 为 JWT 签名密钥（生产环境需替换为安全随机值）
	JWTSecret string
}

// Load 从环境变量加载配置并返回。
func Load() Config {
	return Config{
		ServerAddr:    getEnv("SERVER_ADDR", ":8888"),
		MySQLDSN:      getEnv("MYSQL_DSN", "root:1234@tcp(127.0.0.1:3306)/chat_room?parseTime=true"),
		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnv("REDIS_DB", "0"),
		JWTSecret:     getEnv("JWT_SECRET", "dev_secret_change_me"),
	}
}

// getEnv 获取环境变量，若不存在则返回默认值。
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
