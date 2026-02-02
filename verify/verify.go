package verify

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gin-demo/mailer"

	"github.com/redis/go-redis/v9"

	"github.com/sirupsen/logrus"
)

var (
	defaultTTL = 120 * time.Second
	ctx        = context.Background()
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func genCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func keyForEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("verify:email:%x", sum)
}

// domain whitelist handling
var (
	whitelistOnce sync.Once
	whitelist     []string
)

func loadWhitelist() {
	path := filepath.Join("conf", "whitelist.ini")
	data, err := os.ReadFile(path)
	if err != nil {
		// missing file -> allow all
		whitelist = nil
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}
		whitelist = append(whitelist, strings.ToLower(l))
	}
}

func isDomainAllowed(domain string) bool {
	whitelistOnce.Do(loadWhitelist)
	if len(whitelist) == 0 {
		return true // no whitelist => allow all
	}
	d := strings.ToLower(strings.TrimSpace(domain))
	for _, a := range whitelist {
		if d == a || strings.HasSuffix(d, "."+a) {
			return true
		}
	}
	return false
}

// loadRedisConfig reads conf/redis.ini and returns addr, password and ttl.
func loadRedisConfig() (addr string, password string, ttl time.Duration, err error) {
	path := filepath.Join("conf", "redis.ini")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", defaultTTL, fmt.Errorf("read redis config: %w", err)
	}
	host := ""
	port := ""
	pw := ""
	t := defaultTTL
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
		return "", "", defaultTTL, fmt.Errorf("invalid redis config: host/port missing")
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
		logrus.Errorf("get redis client: load config failed: %v", err)
		return nil, err
	}
	// create a short-lived client per operation (no shared pool)
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
		logrus.Errorf("get redis client: ping failed: %v", err)
		return nil, err
	}
	return rdb, nil
}

// SendCode generates a code, stores it in Redis with TTL from config, and emails it.
func SendCode(email string) error {
	fmt.Println("执行具体的验证码发送和存入redis的操作")
	// check whitelist by recipient domain
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(email)), "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid email")
	}
	domain := parts[1]
	if !isDomainAllowed(domain) {
		return fmt.Errorf("email domain not allowed")
	}

	_, _, ttl, err := loadRedisConfig()
	if err != nil {
		logrus.Errorf("sendcode: load redis config failed: %v", err)
		return err
	}
	code := genCode()
	rdb, err := getRedisClient()
	if err != nil {
		logrus.Errorf("sendcode: get redis client failed: %v", err)
		return err
	}
	// close client when done (short-lived)
	defer func() { _ = rdb.Close() }()
	key := keyForEmail(email)
	// use a short context timeout for Redis operations so requests fail fast
	setCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Set(setCtx, key, code, ttl).Err(); err != nil {
		logrus.Errorf("sendcode: redis set failed: %v", err)
		return err
	}
	subject := "您的验证码"
	body := fmt.Sprintf("您的验证码是: %s。有效期%d秒。", code, int(ttl.Seconds()))
	fmt.Printf("邮件正文: %s\n", body)

	// send mail asynchronously so HTTP handler isn't blocked by SMTP/network
	go func(to, subj, bdy string) {
		if err := mailer.Send(to, subj, bdy); err != nil {
			logrus.Errorf("sendcode: async mail send failed for %s: %v", to, err)
		} else {
			logrus.Infof("sendcode: async mail sent to %s", to)
		}
	}(email, subject, body)

	logrus.Infof("sendcode: queued mail to %s", email)
	return nil
}

// VerifyCode checks a provided code for an email by consulting Redis.
func VerifyCode(email, code string) error {
	_, _, _, err := loadRedisConfig()
	if err != nil {
		logrus.Errorf("verifycode: load redis config failed: %v", err)
		return err
	}
	rdb, err := getRedisClient()
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	key := keyForEmail(email)
	// use short timeout for redis get
	getCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	val, err := rdb.Get(getCtx, key).Result()
	if err == redis.Nil {
		return errors.New("code not found")
	}
	if err != nil {
		logrus.Errorf("verifycode: redis get failed: %v", err)
		return err
	}
	if val != code {
		return errors.New("invalid code")
	}
	// delete once used (non-blocking short timeout)
	delCtx, cancel2 := context.WithTimeout(ctx, 3*time.Second)
	defer cancel2()
	if err := rdb.Del(delCtx, key).Err(); err != nil {
		logrus.Warnf("verifycode: redis del failed: %v", err)
	}
	return nil
}
