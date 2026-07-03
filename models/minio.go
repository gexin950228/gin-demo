package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	conf "gin-demo/conf"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

// InitMinIO 初始化 MinIO 客户端，并确保配置的 bucket 存在（不存在则创建）
// 应在配置加载完成后、路由注册前调用
func InitMinIO() error {
	if conf.MinIOCfg.Endpoint == "" {
		return errors.New("minio endpoint 未配置")
	}
	cli, err := minio.New(conf.MinIOCfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.MinIOCfg.AccessKey, conf.MinIOCfg.SecretKey, ""),
		Secure: conf.MinIOCfg.UseSSL,
	})
	if err != nil {
		return fmt.Errorf("创建 MinIO 客户端失败: %w", err)
	}
	minioClient = cli

	// 确保 bucket 存在
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := minioClient.BucketExists(ctx, conf.MinIOCfg.Bucket)
	if err != nil {
		return fmt.Errorf("查询 MinIO bucket 失败: %w", err)
	}
	if !exists {
		if err := minioClient.MakeBucket(ctx, conf.MinIOCfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("创建 MinIO bucket 失败: %w", err)
		}
	}
	// 无论 bucket 是否新建，都设置公开读策略（已存在的 bucket 可能未配置策略，导致 403）
	// 策略允许匿名用户 GetObject，头像 URL 才能直接访问
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + conf.MinIOCfg.Bucket + `/*"]}]}`
	if err := minioClient.SetBucketPolicy(ctx, conf.MinIOCfg.Bucket, policy); err != nil {
		// 设置策略失败不阻断启动，仅记录
		fmt.Printf("[warn] 设置 bucket 公开读策略失败: %v\n", err)
	}
	return nil
}

// UploadAvatar 上传用户头像到 MinIO
//   - userID: 用户ID，用于生成唯一对象名
//   - file: 文件内容
//   - filename: 原始文件名（用于推断扩展名）
//   - contentType: MIME 类型，如 "image/png"
//
// 返回可访问的头像 URL（基于 PublicURL 拼接）
func UploadAvatar(userID uint, file io.Reader, filename, contentType string) (string, error) {
	if minioClient == nil {
		return "", errors.New("MinIO 客户端未初始化")
	}

	// 生成对象名：avatars/<userID>/<时间戳>.<ext>
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		// 无扩展名时，尝试从 contentType 推断
		exts, _ := mime.ExtensionsByType(contentType)
		if len(exts) > 0 {
			ext = exts[0]
		}
	}
	objectName := fmt.Sprintf("avatars/%d/%d%s", userID, time.Now().UnixMilli(), ext)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := minioClient.PutObject(ctx, conf.MinIOCfg.Bucket, objectName, file, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("上传头像到 MinIO 失败: %w", err)
	}

	// 拼接对外访问 URL
	base := strings.TrimRight(conf.MinIOCfg.PublicURL, "/")
	if base == "" {
		base = fmt.Sprintf("http://%s", conf.MinIOCfg.Endpoint)
	}
	return fmt.Sprintf("%s/%s/%s", base, conf.MinIOCfg.Bucket, objectName), nil
}

// UploadCommentImage 上传评论图片到 MinIO
//   - userID: 评论者用户ID，用于生成唯一对象名
//   - file: 文件内容
//   - filename: 原始文件名（用于推断扩展名）
//   - contentType: MIME 类型，如 "image/png"
//
// 返回可访问的图片 URL（基于 PublicURL 拼接）
func UploadCommentImage(userID uint, file io.Reader, filename, contentType string) (string, error) {
	if minioClient == nil {
		return "", errors.New("MinIO 客户端未初始化")
	}

	// 生成对象名：comments/<userID>/<时间戳>.<ext>
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		exts, _ := mime.ExtensionsByType(contentType)
		if len(exts) > 0 {
			ext = exts[0]
		}
	}
	objectName := fmt.Sprintf("comments/%d/%d%s", userID, time.Now().UnixMilli(), ext)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := minioClient.PutObject(ctx, conf.MinIOCfg.Bucket, objectName, file, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("上传评论图片到 MinIO 失败: %w", err)
	}

	base := strings.TrimRight(conf.MinIOCfg.PublicURL, "/")
	if base == "" {
		base = fmt.Sprintf("http://%s", conf.MinIOCfg.Endpoint)
	}
	return fmt.Sprintf("%s/%s/%s", base, conf.MinIOCfg.Bucket, objectName), nil
}
