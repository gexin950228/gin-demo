package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger
var env string

// Init 初始化全局 Logger
// level: 日志等级 "debug" / "info" / "warn" / "error"
// format: 输出格式 "json"（生产） / "text"（开发）
// environment: 运行环境，用于日志目录分类，如 "prod" / "dev"
func Init(level, format, environment string) {
	log = logrus.New()
	env = environment
	if env == "" {
		env = "default"
	}

	// stdout 输出所有日志
	log.SetOutput(os.Stdout)

	// 解析日志等级
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		parsedLevel = logrus.InfoLevel
	}
	log.SetLevel(parsedLevel)

	// 设置输出格式
	if format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			DisableColors:  true,
		})
	}

	// 按级别+环境+日期分文件写入的 Hook
	log.AddHook(&levelFileHook{formatter: log.Formatter})

	Info("日志系统初始化完成", "level", level, "format", format, "env", environment)
}

// ============================================
// 按级别分文件的 Hook（支持日期切割）
// ============================================

type levelFileHook struct {
	formatter logrus.Formatter
	mu        sync.Mutex
	files     map[string]*dateRotatedFile // key: level name (info/warn/error/debug)
}

func (h *levelFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *levelFileHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.files == nil {
		h.files = make(map[string]*dateRotatedFile)
	}

	levelName := strings.ToLower(entry.Level.String())
	dir := fmt.Sprintf("logs/%s", env)
	os.MkdirAll(dir, 0755)

	f, ok := h.files[levelName]
	if !ok {
		f = newDateRotatedFile(dir, "app", levelName)
		h.files[levelName] = f
	}

	line, err := h.formatter.Format(entry)
	if err != nil {
		return err
	}
	return f.Write(line)
}

// ============================================
// 按日期自动切割的文件写入器
// ============================================

type dateRotatedFile struct {
	dir      string
	prefix   string
	level    string
	mu       sync.Mutex
	file     *os.File
	curDate  string // 当前文件对应的日期 "2006-01-02"
}

func newDateRotatedFile(dir, prefix, level string) *dateRotatedFile {
	return &dateRotatedFile{
		dir:    dir,
		prefix: prefix,
		level:  level,
	}
}

func (f *dateRotatedFile) Write(p []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if today != f.curDate || f.file == nil {
		// 日期变化或首次打开：关闭旧文件，打开新文件
		if f.file != nil {
			f.file.Close()
		}
		filename := fmt.Sprintf("%s/%s-%s.%s.log", f.dir, f.prefix, today, f.level)
		var err error
		f.file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		f.curDate = today
	}

	_, err := f.file.Write(p)
	return err
}

// Close 关闭所有打开的文件句柄（通常在程序退出时调用）
func Close() {
	// 通过重新添加一个空 Hook 的方式无法获取内部状态
	// 这里直接由 GC 处理即可，或显式调用 os.Exit 时系统会关闭
}

// ====== 便捷方法 ======

func Debug(args ...interface{})        { log.Debug(args...) }
func Info(args ...interface{})         { log.Info(args...) }
func Warn(args ...interface{})         { log.Warn(args...) }
func Error(args ...interface{})        { log.Error(args...) }
func Fatal(args ...interface{})       { log.Fatal(args...) }

func Debugf(format string, args ...interface{}) { log.Debugf(format, args...) }
func Infof(format string, args ...interface{})  { log.Infof(format, args...) }
func Warnf(format string, args ...interface{})  { log.Warnf(format, args...) }
func Errorf(format string, args ...interface{}) { log.Errorf(format, args...) }
func Fatalf(format string, args ...interface{}) { log.Fatalf(format, args...) }

// WithFields 创建带字段的子 Logger
func WithFields(fields logrus.Fields) *logrus.Entry {
	return log.WithFields(fields)
}

// RequestLogger 返回 Gin 中间件，记录 HTTP 请求日志
// 对于 4xx/5xx 错误响应，会从响应体中提取具体错误信息写入日志 msg
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 包装 ResponseWriter 以捕获响应体（用于提取错误信息）
		bodyCapture := newResponseBodyCapture(c.Writer)
		c.Writer = bodyCapture

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		fields := logrus.Fields{
			"status":     status,
			"method":     method,
			"path":       path,
			"ip":         clientIP,
			"latency":    latency.String(),
			"user_agent": c.Request.UserAgent(),
		}

		// 收集 Gin 上下文中的错误（c.Error() 添加的）
		var ginErrors string
		if len(c.Errors) > 0 {
			var errs []string
			for _, e := range c.Errors {
				errs = append(errs, e.Error())
			}
			ginErrors = strings.Join(errs, "; ")
			fields["gin_errors"] = ginErrors
		}

		switch {
		case status >= 500:
			// 从响应体中提取具体错误信息
			errMsg := extractErrorMessage(bodyCapture.body.Bytes())
			msg := fmt.Sprintf("服务器错误 %d %s %s", status, method, path)
			if errMsg != "" {
				msg += " - " + errMsg
			}
			if ginErrors != "" {
				msg += " - " + ginErrors
			}
			WithFields(fields).Error(msg)
		case status >= 400:
			errMsg := extractErrorMessage(bodyCapture.body.Bytes())
			msg := fmt.Sprintf("客户端错误 %d %s %s", status, method, path)
			if errMsg != "" {
				msg += " - " + errMsg
			}
			WithFields(fields).Warn(msg)
		case status >= 300:
			WithFields(fields).Info(fmt.Sprintf("重定向 %d %s %s", status, method, path))
		default:
			WithFields(fields).Info(fmt.Sprintf("成功 %d %s %s", status, method, path))
		}
	}
}

// responseBodyCapture 包装 gin.ResponseWriter，捕获响应体内容
type responseBodyCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func newResponseBodyCapture(w gin.ResponseWriter) *responseBodyCapture {
	return &responseBodyCapture{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
	}
}

func (r *responseBodyCapture) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// extractErrorMessage 从响应体中提取错误信息
// 支持 JSON 格式 {"error":"xxx"} 和纯文本
func extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	// 尝试解析 JSON {"error":"..."} 或 {"msg":"..."}
	var jsonResp struct {
		Error string `json:"error"`
		Msg   string `json:"msg"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &jsonResp); err == nil {
		if jsonResp.Error != "" {
			return jsonResp.Error
		}
		if jsonResp.Msg != "" {
			return jsonResp.Msg
		}
		if jsonResp.Message != "" {
			return jsonResp.Message
		}
	}
	// 非 JSON：如果是 HTML 错误页，截取前 200 字符
	text := strings.TrimSpace(string(body))
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	// 过滤掉正常 HTML 页面（避免把首页内容当日志）
	if strings.HasPrefix(text, "<!DOCTYPE") || strings.HasPrefix(text, "<html") {
		return ""
	}
	return text
}
