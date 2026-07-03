// @title Gin Demo API
// @version 1.0
// @description Go Gin 框架示例项目 API 文档，包含用户认证、文章管理、标签管理等接口

// @contact.name API Support
// @contact.url http://localhost:8080/home

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 请在请求头中携带 Token: Bearer <token>
package main

import (
	"flag"
	"fmt"
	"gin-demo/conf"
	_ "gin-demo/docs" // Swagger 生成的文档包
	"gin-demo/handlers/system"
	"gin-demo/logger"
	"gin-demo/models"
	nacosReg "gin-demo/nacos"
	"gin-demo/routers"
	"gin-demo/skywalking"
	"gin-demo/ws"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// 读取命令行参数：-env 指定运行环境（优先级最高）
	env := flag.String("env", "", "运行环境: dev / prod")
	flag.Parse()

	// 优先级：命令行参数 > 环境变量 APP_ENV > 默认 prod
	if *env == "" || (*env != "prod" && *env != "dev") {
		*env = os.Getenv("APP_ENV")
	}
	if *env == "" {
		*env = "prod"
	}

	// 根据环境选择配置文件
	configName := "app-" + *env

	// 配置加载策略：优先从 Nacos 配置中心拉取，失败则回退本地配置文件
	// Nacos 连接信息来自本地 conf/bootstrap.yaml
	if content, err := nacosReg.LoadConfigFromNacos(*env); err == nil {
		conf.InitFromContent(content)
	} else {
		// Nacos 拉取失败时回退本地配置文件（便于离线开发 / Nacos 不可用时仍能启动）
		fmt.Printf("⚠️  从 Nacos 加载配置失败，回退本地配置文件: %v\n", err)
		conf.Init(configName)
	}

	// 初始化日志系统（必须在其他模块之前）
	logger.Init(conf.LogCfg.Level, conf.LogCfg.Format, *env)

	// 预初始化 SkyWalking（必须在 InitDB 之前，否则 GORM 插件注册时 globalTracer 为 nil）
	if conf.SkyWalkingCfg.Enabled {
		if err := skywalking.InitSkyWalking(conf.SkyWalkingCfg.Backend, conf.SkyWalkingCfg.ServiceName, conf.SkyWalkingCfg.ServiceGroup); err != nil {
			logger.Errorf("SkyWalking 初始化失败（非致命）: %v", err)
		} else {
			logger.Info("SkyWalking 链路追踪已启用", "backend", conf.SkyWalkingCfg.Backend)
		}
	}

	// 初始化数据库连接（内部注册 GORM 链路追踪插件，依赖 globalTracer）
	models.InitDB()

	// 初始化Redis连接
	models.InitRedis()

	// 初始化 MinIO 对象存储（用于头像上传）
	if err := models.InitMinIO(); err != nil {
		logger.Errorf("MinIO 初始化失败（非致命）: %v", err)
	} else {
		logger.Info("MinIO 对象存储已启用", "bucket", conf.MinIOCfg.Bucket)
	}

	// 注册服务到 Nacos（在 Gin 启动前注册，失败不中断启动）
	nacosReg.InitNacos()

	// 启动 WebSocket hub 事件循环
	ws.Start()

	// 创建 Gin 引擎并注册 SkyWalking 中间件（必须在路由注册之前）
	r := gin.Default()
	if conf.SkyWalkingCfg.Enabled && skywalking.GetTracer() != nil {
		r.Use(skywalking.Middleware(r))
	}

	// 注册路由
	routers.SetupRoutes(r)

	// 根据配置决定是否注册 Swagger 文档路由
	if conf.SwaggerCfg.Enabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		r.GET("/api-docs", system.APIDocs) // 获取所有接口文档的 JSON 接口
	}

	// 启动服务
	logger.Info("服务启动成功", "port", ":8080")
	r.Run(":8080")
}
