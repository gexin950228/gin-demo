package users

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"

	"gin-demo/auth"
	"gin-demo/models"
	"gin-demo/session"
	"gin-demo/verify"
	"strings"
)

// jwt secret is provided by package auth
var jwtSecret = auth.Secret

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Code     string `json:"code" binding:"required"`
}

// Register handles user registration
func Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("register: bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// verify code before creating user
	if err := verify.VerifyCode(req.Email, req.Code); err != nil {
		logrus.Warnf("register: code verify failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}

	if err := models.CreateUser(req.Username, req.Email, req.Password); err != nil {
		logrus.Warnf("register: create user failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logrus.Infof("user registered: %s", req.Username)
	c.JSON(http.StatusCreated, gin.H{"message": "registered"})
}

// Login handles user login and returns a JWT
func Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("login: bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := models.Authenticate(req.Username, req.Password); err != nil {
		logrus.Warnf("login failed for %s: %v", req.Username, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// create JWT token (24h expiry)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": req.Username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokStr, err := token.SignedString(auth.Secret)
	if err != nil {
		logrus.Errorf("token sign error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	// set token cookie (HttpOnly) valid for 24 hours
	c.SetCookie("token", tokStr, 24*3600, "/", "", false, true)
	// create session in redis
	if err := session.CreateSession(tokStr, req.Username, 24*time.Hour); err != nil {
		logrus.Warnf("failed to create session: %v", err)
	}
	logrus.Infof("user logged in: %s", req.Username)
	c.JSON(http.StatusOK, gin.H{"token": tokStr})
}

// SendCode sends an email verification code
func SendCode(c *gin.Context) {
	type sendReq struct {
		Email string `json:"email" binding:"required,email"`
	}
	var req sendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("sendcode: bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("注册的邮箱是:%v\n", req.Email)
	if err := verify.SendCode(req.Email); err != nil {
		logrus.Errorf("sendcode: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "code sent"})
}

// VerifyCode checks verification code validity
func VerifyCode(c *gin.Context) {
	type checkReq struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}
	var req checkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("verifycode: bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := verify.VerifyCode(req.Email, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// Logout deletes the current session (if any) and clears cookie
func Logout(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "no token"})
		return
	}
	if err := session.DeleteSession(token); err != nil {
		logrus.Warnf("logout: delete session failed: %v", err)
	}
	// clear cookie
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
