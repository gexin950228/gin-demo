package models

import (
	"gin-demo/conf"
	"gin-demo/logger"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

// InitRedis 初始化Redis连接
func InitRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     conf.RedisCfg.Host + ":" + conf.RedisCfg.Port,
		Password: conf.RedisCfg.Password,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		logger.Warn("Redis连接失败", "error", err, "host", conf.RedisCfg.Host+":"+conf.RedisCfg.Port)
		logger.Warn("某些功能（验证码、Token认证）将无法使用")
		return
	}

	logger.Info("Redis连接成功")
}

// SetVerificationCode 存储验证码到Redis（5分钟过期）
func SetVerificationCode(email, code string) error {
	key := "verify_code:" + email
	return RDB.Set(context.Background(), key, code, 5*time.Minute).Err()
}

// GetVerificationCode 获取验证码
func GetVerificationCode(email string) (string, error) {
	key := "verify_code:" + email
	return RDB.Get(context.Background(), key).Result()
}

// DeleteVerificationCode 删除验证码
func DeleteVerificationCode(email string) error {
	key := "verify_code:" + email
	return RDB.Del(context.Background(), key).Err()
}

// SetToken 存储用户token到Redis（24小时过期）
func SetToken(token string, userID uint, username string) error {
	key := "token:" + token
	return RDB.Set(context.Background(), key, fmt.Sprintf("%d:%s", userID, username), 24*time.Hour).Err()
}

// GetToken 获取token对应的用户信息
func GetToken(token string) (uint, string, error) {
	key := "token:" + token
	result, err := RDB.Get(context.Background(), key).Result()
	if err != nil {
		return 0, "", err
	}

	var userID uint
	var username string
	fmt.Sscanf(result, "%d:%s", &userID, &username)
	return userID, username, nil
}

// DeleteToken 删除token（登出时使用）
func DeleteToken(token string) error {
	key := "token:" + token
	return RDB.Del(context.Background(), key).Err()
}

// PingRedis 检查Redis连接是否正常
func PingRedis() error {
	if RDB == nil {
		return fmt.Errorf("Redis客户端未初始化")
	}
	_, err := RDB.Ping(context.Background()).Result()
	return err
}
