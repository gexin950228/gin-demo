package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"gin-demo/auth"

	"github.com/sirupsen/logrus"
)

var ctx = context.Background()

func keyForToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("session:token:%s", hex.EncodeToString(sum[:]))
}

func loadRedisConfig() (addr string, password string, ttl time.Duration, err error) {
	path := filepath.Join("conf", "redis.ini")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", 0, fmt.Errorf("read redis config: %w", err)
	}
	host := ""
	port := ""
	pw := ""
	t := 120 * time.Second
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(parts[0]))
		v := strings.TrimSpace(parts[1])
		switch k {
		case "host":
			host = v
		case "port":
			port = v
		case "password":
			pw = v
		case "ttl":
			if s, err := strconv.Atoi(v); err == nil && s > 0 {
				t = time.Duration(s) * time.Second
			}
		}
	}
	if host == "" && port == "" {
		return "", "", 0, fmt.Errorf("invalid redis config: host/port missing")
	}
	if host != "" && strings.Contains(host, ":") {
		addr = host
	} else {
		addr = host
		if port != "" {
			addr = addr + ":" + port
		}
	}
	return addr, pw, t, nil
}

func getRedisClient() (*redis.Client, error) {
	addr, pw, _, err := loadRedisConfig()
	if err != nil {
		logrus.Errorf("session: load redis config failed: %v", err)
		return nil, err
	}
	// create a short-lived client per operation
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     pw,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		logrus.Errorf("session: ping redis failed: %v", err)
		return nil, err
	}
	return rdb, nil
}

// CreateSession stores token->username mapping in redis with TTL
func CreateSession(token string, username string, ttl time.Duration) error {
	rdb, err := getRedisClient()
	if err != nil {
		logrus.Warnf("session: create - failed to get redis client: %v", err)
		return err
	}
	// close client when done
	defer func() { _ = rdb.Close() }()
	key := keyForToken(token)
	// use a short timeout for the set
	setCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Set(setCtx, key, username, ttl).Err(); err != nil {
		logrus.Warnf("session: create - set failed key=%s user=%s err=%v", key, username, err)
		return err
	}
	logrus.Infof("session: created key=%s user=%s ttl=%s", key, username, ttl.String())
	return nil
}

// ValidateSession checks token exists in redis and returns associated username
func ValidateSession(token string) (string, error) {
	username := ""
	rdb, err := getRedisClient()
	if err != nil {
		logrus.Warnf("session: validate - failed to get redis client: %v", err)
		return "", err
	}
	// close client when done
	defer func() { _ = rdb.Close() }()
	key := keyForToken(token)
	// use short timeout for get
	getCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	val, err := rdb.Get(getCtx, key).Result()
	if err == redis.Nil {
		logrus.Warnf("session: validate - not found key=%s", key)
		return "", fmt.Errorf("session not found")
	}
	if err != nil {
		logrus.Warnf("session: validate - get failed key=%s err=%v", key, err)
		return "", err
	}
	username = val
	logrus.Infof("session: validate - key=%s user=%s", key, username)
	return username, nil
}

// DeleteSession removes session from redis
func DeleteSession(token string) error {
	rdb, err := getRedisClient()
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	key := keyForToken(token)
	return rdb.Del(ctx, key).Err()
}

// AuthRequired is a Gin middleware that validates token signature and session presence
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// token from cookie or Authorization header
		token := ""
		if t, err := c.Cookie("token"); err == nil {
			token = t
		}
		if token == "" {
			ah := c.GetHeader("Authorization")
			if strings.HasPrefix(ah, "Bearer ") {
				token = strings.TrimPrefix(ah, "Bearer ")
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing token"})
			return
		}

		// verify token signature and extract subject
		user, err := auth.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}

		// check session
		uname, err := ValidateSession(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid session"})
			return
		}
		if uname != user {
			c.AbortWithStatusJSON(401, gin.H{"error": "session user mismatch"})
			return
		}

		// store user in context
		c.Set("user", user)
		c.Next()
	}
}

// GlobalAuthMiddleware enforces login for all non-user pages.
// It skips paths under /users, /static, /health, and /articles and redirects browser GETs to /users/to_login.
func GlobalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// whitelist
		if strings.HasPrefix(path, "/users") || strings.HasPrefix(path, "/static") || path == "/health" || strings.HasPrefix(path, "/favicon") || strings.HasPrefix(path, "/articles") {
			logrus.Infof("GlobalAuthMiddleware: skipping auth for path: %s", path)
			c.Next()
			return
		}
		logrus.Infof("GlobalAuthMiddleware: checking auth for path: %s", path)

		// check token
		token := ""
		if t, err := c.Cookie("token"); err == nil {
			token = t
		}
		if token == "" {
			ah := c.GetHeader("Authorization")
			if strings.HasPrefix(ah, "Bearer ") {
				token = strings.TrimPrefix(ah, "Bearer ")
			}
		}
		if token == "" {
			// if browser GET with HTML accept, let page load and let client-side guard handle redirect
			if c.Request.Method == http.MethodGet && strings.Contains(c.GetHeader("Accept"), "text/html") {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		user, err := auth.ParseToken(token)
		if err != nil {
			// allow HTML pages to be served; client will check token from localStorage
			if c.Request.Method == http.MethodGet && strings.Contains(c.GetHeader("Accept"), "text/html") {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		uname, err := ValidateSession(token)
		if err != nil || uname != user {
			// allow HTML pages to be served and let client guard handle redirect
			if c.Request.Method == http.MethodGet && strings.Contains(c.GetHeader("Accept"), "text/html") {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		// authenticated
		c.Set("user", user)
		c.Next()
	}
}
