package conf

import (
	"bytes"
	"fmt"

	"github.com/spf13/viper"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost string
	SMTPPort int
	From     string
	Password string
}

// SwaggerConfig Swagger文档配置
type SwaggerConfig struct {
	Enabled bool
}

// SkyWalkingConfig 链路追踪配置
type SkyWalkingConfig struct {
	Enabled      bool
	Backend      string // OAP gRPC 地址，如 "192.168.1.100:11800"
	ServiceName  string // 服务名称
	ServiceGroup string // 服务分组
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string // debug / info / warn / error
	Format string // json / text
}

// MinIOConfig 对象存储配置
type MinIOConfig struct {
	Endpoint        string // MinIO API 地址，如 "172.22.222.113:9000"
	ConsoleEndpoint string // MinIO 控制台地址（仅参考，不参与连接）
	AccessKey       string
	SecretKey       string
	Bucket          string // 头像存储桶名，如 "userIcon"
	UseSSL          bool   // 是否使用 HTTPS
	PublicURL       string // 拼接头像访问 URL 的公网/对外地址，如 "http://172.22.222.113:9000"
}

// NacosConfig 服务注册与发现配置
type NacosConfig struct {
	Enabled     bool   // 是否启用 Nacos 注册
	ServerHost  string // Nacos 服务端地址
	ServerPort  uint64 // Nacos 服务端端口
	Username    string // Nacos 认证用户名
	Password    string // Nacos 认证密码
	NamespaceID string // 命名空间 ID
	Group       string // 服务分组
	ServiceName string // 注册的服务名称
	Cluster     string // 集群名称
	ServicePort uint64 // 服务监听端口
	ServiceIP   string // 注册 IP，留空则自动获取本机内网 IP
}

// 全局配置变量（由 Init() 从配置文件填充）
var (
	DBConfig        DatabaseConfig
	ArticleDBConfig DatabaseConfig
	RedisCfg        RedisConfig
	EmailConf       EmailConfig
	SwaggerCfg      SwaggerConfig
	SkyWalkingCfg   SkyWalkingConfig
	LogCfg          LogConfig
	MinIOCfg        MinIOConfig
	NacosCfg        NacosConfig
)

// Init 读取 conf/ 下的 YAML 配置文件并初始化全局配置变量
// configName 为配置文件名（不含扩展名），如 "app-dev"、"app-prod"
// 必须在数据库、Redis、邮件等模块初始化之前调用
func Init(configName string) {
	if configName == "" {
		configName = "app" // 默认值
	}
	viper.SetConfigName(configName) // 配置文件名（不含扩展名）
	viper.SetConfigType("yaml")     // 配置文件类型
	viper.AddConfigPath("conf")
	viper.AddConfigPath("./conf")
	viper.AddConfigPath("../conf")

	if err := viper.ReadInConfig(); err != nil {
		panic("读取配置文件失败: " + err.Error())
	}

	parseConfig()
	fmt.Println("✅ 配置文件加载成功:", viper.ConfigFileUsed())
}

// InitFromContent 从 YAML 配置内容字符串初始化全局配置变量
// 用于从 Nacos 配置中心拉取到的配置内容解析（不依赖本地文件）
// content 为 YAML 格式的配置文本
func InitFromContent(content string) {
	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(bytes.NewReader([]byte(content))); err != nil {
		panic("解析配置内容失败: " + err.Error())
	}
	parseConfig()
	fmt.Println("✅ 配置加载成功（来源：Nacos 配置中心）")
}

// parseConfig 从已加载到 viper 的配置中填充全局配置变量
// 供 Init（本地文件）和 InitFromContent（Nacos 内容）复用
func parseConfig() {
	// 读取 MySQL 配置
	DBConfig = DatabaseConfig{
		Host:     viper.GetString("mysql.host"),
		Port:     viper.GetString("mysql.port"),
		User:     viper.GetString("mysql.user"),
		Password: viper.GetString("mysql.password"),
		DBName:   viper.GetString("mysql.dbname"),
	}

	// 读取文章库 MySQL 配置：优先读取 mysql_articles 配置块，
	// 若未配置则 fallback 复用 mysql 的 host/port/user/password，仅 dbname 改为 "articles"
	ArticleDBConfig = DatabaseConfig{
		Host:     viper.GetString("mysql_articles.host"),
		Port:     viper.GetString("mysql_articles.port"),
		User:     viper.GetString("mysql_articles.user"),
		Password: viper.GetString("mysql_articles.password"),
		DBName:   viper.GetString("mysql_articles.dbname"),
	}
	if ArticleDBConfig.Host == "" {
		ArticleDBConfig = DatabaseConfig{
			Host:     DBConfig.Host,
			Port:     DBConfig.Port,
			User:     DBConfig.User,
			Password: DBConfig.Password,
			DBName:   "articles",
		}
	}

	// 读取 Redis 配置
	RedisCfg = RedisConfig{
		Host:     viper.GetString("redis.host"),
		Port:     viper.GetString("redis.port"),
		Password: viper.GetString("redis.password"),
	}

	// 读取邮件配置
	EmailConf = EmailConfig{
		SMTPHost: viper.GetString("email.smtp_host"),
		SMTPPort: viper.GetInt("email.smtp_port"),
		From:     viper.GetString("email.from"),
		Password: viper.GetString("email.password"),
	}

	// 读取 Swagger 配置
	SwaggerCfg = SwaggerConfig{
		Enabled: viper.GetBool("swagger.enabled"),
	}

	// 读取 SkyWalking 配置
	SkyWalkingCfg = SkyWalkingConfig{
		Enabled:      viper.GetBool("skywalking.enabled"),
		Backend:      viper.GetString("skywalking.backend"),
		ServiceName:  viper.GetString("skywalking.service_name"),
		ServiceGroup: viper.GetString("skywalking.service_group"),
	}

	// 读取日志配置
	LogCfg = LogConfig{
		Level:  viper.GetString("log.level"),
		Format: viper.GetString("log.format"),
	}

	// 读取 MinIO 配置
	MinIOCfg = MinIOConfig{
		Endpoint:        viper.GetString("minio.endpoint"),
		ConsoleEndpoint: viper.GetString("minio.console_endpoint"),
		AccessKey:       viper.GetString("minio.access_key"),
		SecretKey:       viper.GetString("minio.secret_key"),
		Bucket:          viper.GetString("minio.bucket"),
		UseSSL:          viper.GetBool("minio.use_ssl"),
		PublicURL:       viper.GetString("minio.public_url"),
	}

	// 读取 Nacos 配置
	NacosCfg = NacosConfig{
		Enabled:     viper.GetBool("nacos.enabled"),
		ServerHost:  viper.GetString("nacos.server_host"),
		ServerPort:  viper.GetUint64("nacos.server_port"),
		Username:    viper.GetString("nacos.username"),
		Password:    viper.GetString("nacos.password"),
		NamespaceID: viper.GetString("nacos.namespace_id"),
		Group:       viper.GetString("nacos.group"),
		ServiceName: viper.GetString("nacos.service_name"),
		Cluster:     viper.GetString("nacos.cluster"),
		ServicePort: viper.GetUint64("nacos.service_port"),
		ServiceIP:   viper.GetString("nacos.service_ip"),
	}
}
