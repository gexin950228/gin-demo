package main

import (
	"gin-demo/conf"
	"gin-demo/models"
	"fmt"
	"os"
)

func main() {
	// 读取配置文件
	conf.Init("") // 使用默认配置 app.yaml

	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Println("       Gin Demo 邮箱配置测试工具")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Println()

	// 测试邮箱连接
	err := models.TestEmailConnection()
	if err != nil {
		fmt.Printf("\n❌ 邮箱连接测试失败:\n")
		fmt.Printf("   错误: %v\n\n", err)
		fmt.Println("请检查以下事项:")
		fmt.Println("  1. 确认163邮箱已开启SMTP服务")
		fmt.Println("  2. 确认授权码正确（不是登录密码）")
		fmt.Println("  3. 确认网络可以访问 smtp.163.com:465")
		fmt.Println("  4. 检查防火墙是否阻止了出站连接")
		os.Exit(1)
	}

	os.Exit(0)
}
