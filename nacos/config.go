package nacos

import (
	"fmt"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/viper"
)

// bootstrapCfg 引导配置（仅 Nacos 连接信息），从本地 conf/bootstrap.yaml 读取
type bootstrapCfg struct {
	ServerHost   string
	ServerPort   uint64
	Username     string
	Password     string
	NamespaceID  string
	Group        string
	DataIdPrefix string // dataId 前缀，最终 dataId = <prefix>-<env>.yaml
}

// LoadBootstrap 从本地 conf/bootstrap.yaml 读取 Nacos 连接信息
// 失败则 panic（没有引导配置无法连接配置中心）
func LoadBootstrap() *bootstrapCfg {
	v := viper.New()
	v.SetConfigName("bootstrap")
	v.SetConfigType("yaml")
	v.AddConfigPath("conf")
	v.AddConfigPath("./conf")
	v.AddConfigPath("../conf")
	if err := v.ReadInConfig(); err != nil {
		panic("读取引导配置 conf/bootstrap.yaml 失败: " + err.Error())
	}

	cfg := &bootstrapCfg{
		ServerHost:   v.GetString("nacos.server_host"),
		ServerPort:   v.GetUint64("nacos.server_port"),
		Username:     v.GetString("nacos.username"),
		Password:     v.GetString("nacos.password"),
		NamespaceID:  v.GetString("nacos.namespace_id"),
		Group:        v.GetString("nacos.group"),
		DataIdPrefix: v.GetString("nacos.data_id_prefix"),
	}
	if cfg.ServerHost == "" {
		panic("引导配置缺少 nacos.server_host")
	}
	if cfg.Group == "" {
		cfg.Group = "DEFAULT_GROUP"
	}
	if cfg.DataIdPrefix == "" {
		cfg.DataIdPrefix = "app"
	}
	return cfg
}

// LoadConfigFromNacos 从 Nacos 配置中心拉取指定环境的配置内容
// env 为运行环境（dev / prod），最终 dataId = <prefix>-<env>.yaml
// 返回 YAML 配置文本，供 conf.InitFromContent 解析
func LoadConfigFromNacos(env string) (string, error) {
	bs := LoadBootstrap()

	// 1. 构建 ClientConfig
	clientConfig := constant.ClientConfig{
		NamespaceId:         bs.NamespaceID,
		Username:            bs.Username,
		Password:            bs.Password,
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "logs/nacos",
		CacheDir:            "logs/nacos/cache",
		LogLevel:            "info",
		AppendToStdout:      true,
	}

	// 2. 构建 ServerConfig
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr:      bs.ServerHost,
			Port:        bs.ServerPort,
			Scheme:      "http",
			ContextPath: "/nacos",
		},
	}

	// 3. 创建 Config 客户端
	configClient, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return "", fmt.Errorf("创建 Nacos Config 客户端失败: %w", err)
	}

	// 4. 拉取配置
	dataId := fmt.Sprintf("%s-%s.yaml", bs.DataIdPrefix, env)
	content, err := configClient.GetConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  bs.Group,
	})
	if err != nil {
		return "", fmt.Errorf("从 Nacos 拉取配置失败 (dataId=%s, group=%s, namespace=%s): %w",
			dataId, bs.Group, bs.NamespaceID, err)
	}
	if content == "" {
		return "", fmt.Errorf("Nacos 配置内容为空 (dataId=%s, group=%s, namespace=%s)，请先在 Nacos 控制台创建该配置",
			dataId, bs.Group, bs.NamespaceID)
	}

	// logger 此时可能尚未初始化，用 fmt 输出（main.go 在 logger.Init 之前调用本函数）
	fmt.Printf("✅ 从 Nacos 配置中心加载配置成功: dataId=%s group=%s namespace=%s size=%d\n",
		dataId, bs.Group, bs.NamespaceID, len(content))
	return content, nil
}
