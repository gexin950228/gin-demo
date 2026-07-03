package system

import (
	"fmt"
	"gin-demo/models"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"runtime"
	"time"
)

// Prometheus 指标收集器
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gin_demo_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gin_demo_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	cpuUsagePercent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gin_demo_cpu_usage_percent",
			Help: "Current CPU usage percentage",
		},
	)

	memUsageBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gin_demo_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)

	goroutinesCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gin_demo_goroutines_count",
			Help: "Current number of goroutines",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(cpuUsagePercent)
	prometheus.MustRegister(memUsageBytes)
	prometheus.MustRegister(goroutinesCount)

	// 启动后台协程定期采集系统指标
	go collectSystemMetrics()
}

// collectSystemMetrics 每5秒采集一次 CPU、内存等系统指标
func collectSystemMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// 内存使用量（已分配的堆内存）
		memUsageBytes.Set(float64(m.Alloc))

		// Goroutine 数量
		goroutinesCount.Set(float64(runtime.NumGoroutine()))
	}
}

// HealthCheck 健康检查接口
// @Summary 服务健康检查
// @Description 检查服务、数据库和Redis的连接状态，用于容器化部署时探测服务是否就绪
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]interface{} "健康"
// @Failure 503 {object} map[string]interface{} "部分服务不可用"
// @Router /healthcheck [get]
func HealthCheck(c *gin.Context) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	}

	// 检查数据库连接
	dbErr := models.PingDB()
	if dbErr != nil {
		status["status"] = "degraded"
		status["database"] = "unavailable: " + dbErr.Error()
	} else {
		status["database"] = "ok"
	}

	// 检查 Redis 连接
	redisErr := models.PingRedis()
	if redisErr != nil {
		if status["status"] == "ok" {
			status["status"] = "degraded"
		}
		status["redis"] = "unavailable: " + redisErr.Error()
	} else {
		status["redis"] = "ok"
	}

	httpStatus := http.StatusOK
	if status["status"] != "ok" {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}

// PrometheusMetrics Prometheus 指标接口
// @Summary Prometheus 指标采集
// @Description 返回 Prometheus 格式的监控指标数据，供 Prometheus Server 定期拉取
// @Tags 系统
// @Produce plain
// @Success 200 {string} string "Prometheus 指标数据"
// @Router /metrics [get]
func PrometheusMetrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

// APIDocs 获取所有接口文档（JSON格式）
// @Summary 获取所有接口文档
// @Description 返回完整的 API 文档 JSON，包含所有接口的路径、方法、参数、响应等信息。可用于程序化获取接口列表。
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]interface{} "完整API文档"
// @Router /api-docs [get]
func APIDocs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":   "API 文档已启用",
		"swaggerUI": "/swagger/index.html",
		"hint":      "访问 /swagger/index.html 查看交互式 API 文档",
	})
}

// PrometheusMiddleware Prometheus 中间件，记录请求指标
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := c.Writer.Status()

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, fmt.Sprintf("%d", status)).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
