package skywalking

import (
	"context"
	"net/http"

	"github.com/SkyAPM/go2sky"
	"github.com/SkyAPM/go2sky/reporter"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	swgin "github.com/SkyAPM/go2sky-plugins/gin/v3"
)

// InitSkyWalking 初始化 SkyWalking Agent
// serviceGroup 通过 SkyWalking 的 "<组名>::<服务名>" 格式设置，
// OAP 服务器会自动解析 "::" 分隔符并将组名分配到服务元数据中。
func InitSkyWalking(backend, serviceName, serviceGroup string) error {
	rpt, err := reporter.NewGRPCReporter(backend)
	if err != nil {
		return err
	}

	// SkyWalking Service Group 机制：服务名采用 "<组名>::<服务名>" 格式
	// OAP 服务器解析 "::" 后将组名写入服务元数据的 group 字段
	// 参考: https://skywalking.apache.org/docs/skywalking-java/latest/en/setup/service-agent/java-agent/configurations/
	fullServiceName := serviceName
	if serviceGroup != "" {
		fullServiceName = serviceGroup + "::" + serviceName
	}

	tracer, err := go2sky.NewTracer(fullServiceName, go2sky.WithReporter(rpt))
	if err != nil {
		return err
	}

	globalTracer = tracer
	return nil
}

var globalTracer *go2sky.Tracer

// GetTracer 获取全局 Tracer 实例
func GetTracer() *go2sky.Tracer {
	return globalTracer
}

// Middleware 返回 Gin 中间件，自动采集所有 HTTP 请求的链路数据
// 必须在路由注册之前调用：
//
//	r := gin.Default()
//	skywalking.InitSkyWalking("oap:11800", "gin-demo")
//	r.Use(skywalking.Middleware(r))
//	routers.SetupRoutes(r)
func Middleware(engine *gin.Engine) gin.HandlerFunc {
	if globalTracer == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return swgin.Middleware(engine, globalTracer)
}

// ============================================
// 子 Span 创建（自动关联到同一 Trace）
// ============================================

// NewSpan 创建一个子 Span，自动关联到当前请求的同一 Trace
//
// 原理：swgin.Middleware 已将 Entry Span 的上下文注入 c.Request.Context()，
//       NewSpan 从中提取父级信息，创建的子 Span 自动归属同一 trace_id。
//
// 使用方式：
//
//	defer skywalking.NewSpan(c, "redis:验证Token").End()
//
// 返回 nil 时（SkyWalking 未启用），安全调用 .End() 不会 panic。
func NewSpan(c *gin.Context, operationName string) go2sky.Span {
	if globalTracer == nil || !IsEnabled() {
		return nil
	}
	// c.Request.Context() 已包含 swgin 注入的 Trace 上下文（trace_id/span_id 等）
	// 使用 CreateLocalSpan 创建内部子 Span，自动归属同一 trace_id
	span, _, err := globalTracer.CreateLocalSpan(
		c.Request.Context(),
		go2sky.WithOperationName(operationName),
	)
	if err != nil {
		return nil
	}
	return span
}

// ============================================
// GORM 自动打点插件（零侵入，关联到同一 Trace）
// ============================================

// GormPlugin 返回 GORM 插件，自动为每条 SQL 创建子 Span
// 使用方式：DB.Use(skywalking.GormPlugin())
//
// Handler 中需配合 WithTraceContext(c) 使用：
//
//	ctx := skywalking.WithTraceContext(c)
//	models.DB.WithContext(ctx).Find(&articles)
func GormPlugin() gorm.Plugin {
	if globalTracer == nil {
		return &nopPlugin{}
	}
	return &gormTracePlugin{}
}

type gormTracePlugin struct{}

func (p *gormTracePlugin) Name() string { return "skywalking-trace" }

func (p *gormTracePlugin) Initialize(db *gorm.DB) error {
	db.Callback().Query().Before("gorm:query").Register("sw:before_query", p.beforeQuery)
	db.Callback().Query().After("gorm:query").Register("sw:after_query", p.afterQuery)
	db.Callback().Create().Before("gorm:create").Register("sw:before_create", p.beforeQuery)
	db.Callback().Create().After("gorm:create").Register("sw:after_create", p.afterQuery)
	db.Callback().Update().Before("gorm:update").Register("sw:before_update", p.beforeQuery)
	db.Callback().Update().After("gorm:update").Register("sw:after_update", p.afterQuery)
	db.Callback().Delete().Before("gorm:delete").Register("sw:before_delete", p.beforeQuery)
	db.Callback().Delete().After("gorm:delete").Register("sw:after_delete", p.afterQuery)
	return nil
}

func (p *gormTracePlugin) beforeQuery(db *gorm.DB) {
	if globalTracer == nil || !IsEnabled() {
		return
	}
	if db.Statement == nil {
		return
	}

	tableName := db.Statement.Table
	opName := "mysql:" + tableName

	var span go2sky.Span
	var err error

	if db.Statement.Context != nil {
		span, _, err = globalTracer.CreateLocalSpan(
			db.Statement.Context,
			go2sky.WithOperationName(opName),
		)
	}

	if span == nil || err != nil {
		span, _, _ = globalTracer.CreateLocalSpan(context.Background(), go2sky.WithOperationName(opName))
	}

	db.InstanceSet("sw:span", span)
}

func (p *gormTracePlugin) afterQuery(db *gorm.DB) {
	val, ok := db.InstanceGet("sw:span")
	if !ok {
		return
	}
	span, ok := val.(go2sky.Span)
	if !ok || span == nil {
		return
	}

	// After 回调时 SQL 已生成，更新 Span 名称为完整 SQL 语句
	if db.Statement != nil && db.Statement.SQL.String() != "" {
		sql := truncateSQL(db.Statement.SQL.String(), 200)
		span.SetOperationName("mysql:" + sql)
	}

	span.End()
}

type nopPlugin struct{}

func (p *nopPlugin) Name() string             { return "nop" }
func (p *nopPlugin) Initialize(db *gorm.DB) error { return nil }

// ============================================
// 上下文传播工具
// ============================================

// swCtxKey 是 context.Context 中标记"已包含 SkyWalking 链路上下文"的 key
type swCtxKey struct{}

// WithTraceContext 将当前请求的 SkyWalking 链路上下文注入到一个新的 context
// 配合 models.DB.WithContext(ctx) 使用，让 GORM 回调中的 SQL 操作能关联到同一 Trace。
//
// 使用方式：
//
//	ctx := skywalking.WithTraceContext(c)
//	models.DB.WithContext(ctx).Preload("User").Find(&articles)
func WithTraceContext(c *gin.Context) context.Context {
	// 标记此 context 包含 SkyWalking 链路上下文（GORM 插件据此判断是否创建关联 Span）
	return context.WithValue(c.Request.Context(), swCtxKey{}, true)
}

// HasTraceContext 检查 context 是否已携带 SkyWalking 链路上下文
func HasTraceContext(ctx context.Context) bool {
	return ctx.Value(swCtxKey{}) != nil
}

// ============================================
// 动态开关控制
// ============================================

var swEnabled = true

// SetEnabled 动态开启/关闭链路追踪上报
func SetEnabled(enabled bool) {
	swEnabled = enabled
}

// IsEnabled 查询当前开关状态
func IsEnabled() bool {
	return swEnabled && globalTracer != nil
}

// ToggleHandler 切换 SkyWalking 开关的管理接口（仅 user_id=1）
// POST /system/skywalking/toggle   Body: {"enabled": true|false}
func ToggleHandler(c *gin.Context) {
	if c.GetUint("user_id") != 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "仅管理员可操作"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: 需要 enabled 字段 (bool)"})
		return
	}

	SetEnabled(req.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"msg":     "已切换",
		"enabled": req.Enabled,
	})
}

// truncateSQL 截断过长的 SQL 用于 Span 名称展示
func truncateSQL(sql string, maxLen int) string {
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}
