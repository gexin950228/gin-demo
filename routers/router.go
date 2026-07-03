package routers

import (
	"gin-demo/handlers/articles"
	"gin-demo/handlers/system"
	"gin-demo/logger"
	"gin-demo/middleware"
	"gin-demo/routes"
	"gin-demo/skywalking"
	"gin-demo/ws"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// SetupRoutes 注册所有路由到传入的 gin.Engine
// 调用方应在调用此函数前完成：
//   - 创建 engine (gin.Default())
//   - 挂载需要全局生效的中间件（如 SkyWalking 链路追踪）
func SetupRoutes(r *gin.Engine) {
	// 加载HTML模板（注册自定义模板函数）
	r.SetFuncMap(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"buildPageList": buildPageList,
	})
	// 按 templates/ 下路由分组子目录递归加载所有 .html 文件
	// 模板名用文件 basename（如 user/login.html → "login.html"），handler 仍用文件名引用
	r.LoadHTMLFiles(loadTemplateFiles("templates")...)

	// 静态文件
	r.Static("/static", "./static")

	// 全局中间件
	r.Use(logger.RequestLogger())                    // 请求日志（记录方法/路径/状态码/耗时）
	r.Use(system.PrometheusMiddleware())            // Prometheus 指标采集

	// 系统接口（无需认证）
	r.GET("/healthcheck", system.HealthCheck)
	r.GET("/metrics", system.PrometheusMetrics)

	// SkyWalking 动态开关（仅管理员 user_id=1）
	r.POST("/system/skywalking/toggle", skywalking.ToggleHandler)

	// WebSocket 端点（在 handler 内部通过 cookie 认证，不走 AuthMiddleware）
	r.GET("/ws", ws.HandleWS)

	// 公开路由（不需要认证）
	public := r.Group("/")
	SetupUserRoutes(public)
	// 根路径重定向到 /home（由 AuthMiddleware 处理未登录跳转）
	public.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, routes.Reverse(routes.Home))
	})

	// 需要认证的路由（基于Token）
	auth := r.Group("/")
	auth.Use(middleware.AuthMiddleware())
	{
		auth.GET("/home", articles.HomePage)

		// 文章路由组
		SetupArticleRoutes(auth)

		// 审批路由组
		SetupApprovalRoutes(auth)

		// 已登录用户路由组（头像上传等）
		SetupUserAuthRoutes(auth)
	}
}

// buildPageList 生成要显示的页码列表（超过7页时用 "..." 表示省略）
// 返回字符串切片，元素为页码字符串或 "..."
func buildPageList(current, total int) []string {
	result := []string{}
	if total <= 7 {
		for i := 1; i <= total; i++ {
			result = append(result, strconv.Itoa(i))
		}
		return result
	}
	result = append(result, "1")
	start := current - 1
	if start < 2 {
		start = 2
	}
	end := current + 1
	if end > total-1 {
		end = total - 1
	}
	if start > 2 {
		result = append(result, "...")
	}
	for j := start; j <= end; j++ {
		result = append(result, strconv.Itoa(j))
	}
	if end < total-1 {
		result = append(result, "...")
	}
	result = append(result, strconv.Itoa(total))
	return result
}

// loadTemplateFiles 递归收集指定目录下所有 .html 文件路径
// 配合 r.LoadHTMLFiles 使用，模板名默认为文件 basename，handler 仍用文件名引用
func loadTemplateFiles(root string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			files = append(files, path)
		}
		return nil
	})
	return files
}
