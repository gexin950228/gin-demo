package nacos

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"gin-demo/conf"
	"gin-demo/logger"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var namingClient naming_client.INamingClient

// 注册实例参数缓存，用于优雅注销
var registerParam vo.RegisterInstanceParam

// InitNacos 初始化 Nacos Naming 客户端并注册服务实例
// 在服务启动前调用，失败时仅记录日志不中断启动（非致命）
func InitNacos() {
	if !conf.NacosCfg.Enabled {
		logger.Info("Nacos 服务注册未启用（nacos.enabled=false）")
		return
	}

	// ========== 1. 构建 ClientConfig ==========
	clientConfig := constant.ClientConfig{
		NamespaceId:       conf.NacosCfg.NamespaceID,
		Username:          conf.NacosCfg.Username,
		Password:          conf.NacosCfg.Password,
		TimeoutMs:         5000,
		NotLoadCacheAtStart: true,
		LogDir:            "logs/nacos",
		CacheDir:          "logs/nacos/cache",
		LogLevel:          "info",
		AppendToStdout:    true,
		LogRollingConfig: &constant.ClientLogRollingConfig{
			MaxSize:    100, // 日志文件最大 100MB
			MaxAge:     3,   // 保留 3 天
			MaxBackups: 10,  // 保留 10 个备份
			LocalTime:  true,
		},
	}

	// ========== 2. 构建 ServerConfig ==========
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr:      conf.NacosCfg.ServerHost,
			Port:        conf.NacosCfg.ServerPort,
			Scheme:      "http",
			ContextPath: "/nacos",
		},
	}

	// ========== 3. 创建 Naming 客户端 ==========
	var err error
	namingClient, err = clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		logger.Errorf("Nacos Naming 客户端创建失败（非致命）: %v", err)
		return
	}

	// ========== 4. 获取注册 IP ==========
	ip := conf.NacosCfg.ServiceIP
	if ip == "" {
		ip, err = getLocalIP()
		if err != nil {
			logger.Errorf("获取本机内网 IP 失败（非致命）: %v", err)
			return
		}
	}

	// ========== 5. 注册服务实例 ==========
	registerParam = vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        conf.NacosCfg.ServicePort,
		ServiceName: conf.NacosCfg.ServiceName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true, // 临时实例，客户端发送心跳保活
		ClusterName: conf.NacosCfg.Cluster,
		GroupName:   conf.NacosCfg.Group,
		Metadata: map[string]string{
			"framework":                 "gin",
			"version":                   "1.0",
			"namespace":                  conf.NacosCfg.NamespaceID,
			"preserved.register.source": "SPRING_CLOUD",
		},
	}

	success, err := namingClient.RegisterInstance(registerParam)
	if err != nil {
		logger.Errorf("Nacos 服务注册失败（非致命）: %v", err)
		return
	}
	if !success {
		logger.Errorf("Nacos 服务注册返回 false（非致命）")
		return
	}

	logger.Info("Nacos 服务注册成功",
		"service", conf.NacosCfg.ServiceName,
		"ip", ip,
		"port", conf.NacosCfg.ServicePort,
		"namespace", conf.NacosCfg.NamespaceID,
		"group", conf.NacosCfg.Group,
	)

	// ========== 6. 监听退出信号，优雅注销 ==========
	go watchShutdownSignal()
}

// Deregister 从 Nacos 注销服务实例
func Deregister() {
	if namingClient == nil {
		return
	}

	success, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          registerParam.Ip,
		Port:        registerParam.Port,
		ServiceName: registerParam.ServiceName,
		Ephemeral:   true,
		Cluster:     registerParam.ClusterName,
		GroupName:   registerParam.GroupName,
	})
	if err != nil {
		logger.Errorf("Nacos 服务注销失败: %v", err)
		return
	}
	if success {
		logger.Info("Nacos 服务注销成功",
			"service", registerParam.ServiceName,
			"ip", registerParam.Ip,
			"port", registerParam.Port,
		)
	}
}

// watchShutdownSignal 监听系统信号（SIGINT/SIGTERM），收到后优雅注销
func watchShutdownSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("收到退出信号，开始优雅注销 Nacos 服务...", "signal", sig.String())
	Deregister()
	// 注销后给 Nacos 一点时间处理，再退出
	// 实际退出由 main 函数的 r.Run() 被信号中断完成
}

// getLocalIP 获取本机内网 IPv4 地址
// 通过 UDP 拨号方式获取：不需要真正建立连接，只是让操作系统选择出站网卡对应的 IP
func getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("无法确定本机 IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
