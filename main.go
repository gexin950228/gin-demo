package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/session"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// initialize logger (writes leveled logs into separate files under ./logs)
	logDir := "./logs"
	if err := logger.Init(logDir); err != nil {
		panic(err)
	}

	r := gin.New()
	// global auth middleware: enforce login for non-user pages
	r.Use(session.GlobalAuthMiddleware())
	r.Use(gin.Recovery())

	// try loading MySQL config from conf/mysql.ini; fallback to sqlite
	dsn, useMySQL, err := loadMySQLDSN("conf/mysql.ini")
	if err != nil {
		// non-fatal: log and fallback
		// but panic when opening DB fails later
		fmt.Printf("warning: failed to read mysql config: %v\n", err)
	}

	var db *gorm.DB
	if useMySQL {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			panic(err)
		}
	} else {
		db, err = gorm.Open(sqlite.Open("data.db"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
	}
	if err := db.AutoMigrate(&models.User{}, &models.Article{}, &models.Label{}); err != nil {
		panic(err)
	}
	models.InitDB(db)

	// serve static frontend files
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/static/register.html")
	})

	// register routes from routes package (groups /users/*)
	routes.Register(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

// loadMySQLDSN reads a simple key=value ini and returns DSN and whether mysql should be used.
func loadMySQLDSN(path string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	m := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		m[k] = v
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	user, ok1 := m["username"]
	pass, ok2 := m["password"]
	host, ok3 := m["host"]
	port, ok4 := m["port"]
	dbname, ok5 := m["database"]
	if ok1 && ok2 && ok3 && ok4 && ok5 && user != "" {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, dbname)
		return dsn, true, nil
	}
	return "", false, nil
}
