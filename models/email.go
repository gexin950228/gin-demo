package models

import (
	"gin-demo/conf"
	"crypto/tls"
	"fmt"
	"math/rand"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SendVerificationCode 发送验证码邮件
func SendVerificationCode(email, code string) error {
	subject := "【Gin Demo】注册验证码"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #333;">欢迎注册 Gin Demo</h2>
			<p>您的注册验证码是：</p>
			<div style="background: #f0f0f0; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0;">
				%s
			</div>
			<p style="color: #666;">验证码有效期为5分钟，请尽快完成注册。</p>
			<p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
		</div>
	`, code)
	return SendEmail(email, subject, body)
}

// SendLoginVerificationCode 发送登录验证码邮件
func SendLoginVerificationCode(email, code string) error {
	subject := "【Gin Demo】登录验证码"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #333;">登录验证码</h2>
			<p>您的登录验证码是：</p>
			<div style="background: #f0f0f0; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0;">
				%s
			</div>
			<p style="color: #666;">验证码有效期为5分钟，请尽快完成登录。若非本人操作，请忽略此邮件。</p>
			<p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
		</div>
	`, code)
	return SendEmail(email, subject, body)
}

// SendResetPasswordVerificationCode 发送重置密码验证码邮件
func SendResetPasswordVerificationCode(email, code string) error {
	subject := "【Gin Demo】重置密码验证码"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #333;">重置密码验证码</h2>
			<p>您正在重置账户密码，验证码是：</p>
			<div style="background: #f0f0f0; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0;">
				%s
			</div>
			<p style="color: #666;">验证码有效期为5分钟，请尽快完成重置。若非本人操作，请忽略此邮件并建议尽快登录修改密码。</p>
			<p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
		</div>
	`, code)
	return SendEmail(email, subject, body)
}

// SendEmail 通用邮件发送函数
// email: 收件人邮箱
// subject: 邮件主题（支持中文）
// body: 邮件正文（HTML 格式）
func SendEmail(email, subject, body string) error {
	from := conf.EmailConf.From
	password := conf.EmailConf.Password
	to := []string{email}
	host := conf.EmailConf.SMTPHost
	port := conf.EmailConf.SMTPPort

	// Subject 使用 RFC 2047 MIME 编码，解决中文乱码
	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)

	// 构建邮件头
	header := make(map[string]string)
	header["From"] = from
	header["To"] = strings.Join(to, ",")
	header["Subject"] = encodedSubject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"UTF-8\""

	// 构建消息
	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// 使用SSL/TLS连接（163邮箱465端口需要SSL）
	addr := fmt.Sprintf("%s:%d", host, port)

	// 建立TLS连接
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("连接SMTP服务器失败: %v", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %v", err)
	}
	defer client.Close()

	// 认证
	auth := smtp.PlainAuth("", from, password, host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %v", err)
	}

	// 设置发件人和收件人
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %v", err)
	}

	if err = client.Rcpt(to[0]); err != nil {
		return fmt.Errorf("设置收件人失败: %v", err)
	}

	// 发送邮件内容
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取写入流失败: %v", err)
	}

	_, err = writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("关闭写入流失败: %v", err)
	}

	return nil
}

// GenerateRandomCode 生成6位随机验证码
func GenerateRandomCode() string {
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(10000)))
	code := ""
	for i := 0; i < 6; i++ {
		code += fmt.Sprintf("%d", rand.Intn(10))
	}
	return code
}

// TestEmailConnection 测试邮箱连接是否正常
func TestEmailConnection() error {
	host := conf.EmailConf.SMTPHost
	port := conf.EmailConf.SMTPPort
	addr := fmt.Sprintf("%s:%d", host, port)

	fmt.Printf("正在测试邮箱连接: %s ...\n", addr)

	// 测试TCP连接
	timeout := 10 * time.Second
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("无法连接到SMTP服务器 %s: %v (请检查网络和防火墙设置)", addr, err)
	}
	defer conn.Close()

	fmt.Println("✓ TCP连接成功")

	// 测试TLS连接
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	tlsConn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %v (163邮箱需要TLS/SSL支持)", err)
	}
	defer tlsConn.Close()

	fmt.Println("✓ TLS连接成功")

	// 测试SMTP握手
	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		return fmt.Errorf("SMTP客户端创建失败: %v", err)
	}
	defer client.Quit()
	defer client.Close()

	fmt.Println("✓ SMTP握手成功")

	from := conf.EmailConf.From
	password := conf.EmailConf.Password

	auth := smtp.PlainAuth("", from, password, host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %v (请检查邮箱地址和授权码是否正确)", err)
	}

	fmt.Println("✓ SMTP认证成功")
	fmt.Printf("\n✅ 邮箱配置正常，可以正常发送邮件\n")
	fmt.Printf("   发件人: %s\n", from)
	fmt.Printf("   SMTP服务器: %s:%d\n", host, port)

	return nil
}
