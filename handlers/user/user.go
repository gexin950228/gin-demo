package user

import (
	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/skywalking"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// RegisterPage 显示注册页面
// @Summary 注册页面
// @Description 返回用户注册表单页面
// @Tags 用户认证
// @Success 200 {string} string "注册页面HTML"
// @Router /user/register [get]
func RegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title":            "用户注册",
		"url_login":        routes.Reverse(routes.UserLoginPage),
		"url_register":     routes.Reverse(routes.UserRegisterPage),
		"url_send_code":    routes.Reverse(routes.UserSendCode),
		"url_check_username": routes.Reverse(routes.UserCheckUsername),
		"url_check_email":   routes.Reverse(routes.UserCheckEmail),
	})
}

// SendVerificationCode 发送注册验证码
// @Summary 发送邮箱验证码
// @Description 向指定邮箱发送6位数字验证码，用于注册验证。验证码5分钟内有效。
// @Tags 用户认证
// @Accept x-www-form-urlencoded
// @Produce json
// @Param email formData string true "邮箱地址"
// @Success 200 {object} map[string]string "发送成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "发送失败"
// @Router /user/send-code [post]
func SendVerificationCode(c *gin.Context) {
	email := c.PostForm("email")

	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱地址"})
		return
	}

	// 生成6位随机验证码
	code := models.GenerateRandomCode()

	// 存储到Redis（5分钟过期）
	if err := models.SetVerificationCode(email, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码生成失败"})
		return
	}

	// 发送邮件
	if err := models.SendVerificationCode(email, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码发送失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送，请查收邮件"})
}

// CheckUsernameExists 检查用户名是否已存在
// @Summary 检查用户名是否存在
// @Description 检查指定用户名是否已被其他用户使用，用于注册时实时校验
// @Tags 用户认证
// @Produce json
// @Param username query string true "用户名"
// @Success 200 {object} map[string]interface{} "检查结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /user/check-username [get]
func CheckUsernameExists(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名"})
		return
	}

	var count int64
	if err := models.DB.WithContext(ctx).Model(&models.User{}).Where("user_name = ?", username).Count(&count).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"username": username,
			"error":    err,
		}).Warn("CheckUsernameExists 查询用户名失败")
	}

	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"exists": true, "message": "该用户名已被使用"})
	} else {
		c.JSON(http.StatusOK, gin.H{"exists": false, "message": "用户名可用"})
	}
}

// CheckEmailExists 检查邮箱是否已存在
// @Summary 检查邮箱是否存在
// @Description 检查指定邮箱是否已被其他用户注册，用于注册时实时校验
// @Tags 用户认证
// @Produce json
// @Param email query string true "邮箱地址"
// @Success 200 {object} map[string]interface{} "检查结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /user/check-email [get]
func CheckEmailExists(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱地址"})
		return
	}

	var count int64
	if err := models.DB.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"email": email,
			"error": err,
		}).Warn("CheckEmailExists 查询邮箱失败")
	}

	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"exists": true, "message": "该邮箱已被注册"})
	} else {
		c.JSON(http.StatusOK, gin.H{"exists": false, "message": "邮箱可用"})
	}
}

// Register 处理用户注册
// @Summary 用户注册
// @Description 创建新用户账号，需要提供用户名、邮箱、密码（含确认密码）和邮箱验证码
// @Tags 用户认证
// @Accept x-www-form-urlencoded
// @Produce json
// @Param username formData string true "用户名（3-20位字母数字下划线）"
// @Param email formData string true "邮箱地址"
// @Param password formData string true "密码（至少8位，包含2种以上字符类型）"
// @Param confirm_password formData string true "确认密码"
// @Param verify_code formData string true "邮箱验证码（6位数字）"
// @Success 302 {string} string "重定向到登录页"
// @Failure 400 {object} map[string]string "参数错误或格式不合法"
// @Failure 409 {object} map[string]string "用户名或邮箱已存在"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /user/register [post]
func Register(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	username := c.PostForm("username")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")
	verifyCode := c.PostForm("verify_code")

	// 验证输入是否为空
	if username == "" || email == "" || password == "" || confirmPassword == "" || verifyCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写完整信息"})
		return
	}

	// 验证用户名格式
	if usernameResult := ValidateUsernameFormat(username); !usernameResult.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": usernameResult.Message})
		return
	}

	// 验证邮箱格式
	if emailResult := ValidateEmailFormat(email); !emailResult.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": emailResult.Message})
		return
	}

	// 检查两次密码是否一致
	if password != confirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的密码不一致"})
		return
	}

	// 验证密码强度（至少达到弱级别以上）
	strength := ValidatePasswordStrength(password)
	if strength.Score < 2 { // 至少满足2项规则
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "密码太简单，请使用至少8位密码，并包含以下2种及以上字符类型：大写字母、小写字母、数字、特殊字符",
			"strength": strength,
		})
		return
	}

	// 检查用户名是否已存在
	var usernameCount int64
	if err := models.DB.WithContext(ctx).Model(&models.User{}).Where("user_name = ?", username).Count(&usernameCount).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"username": username,
			"error":    err,
		}).Warn("Register 查询用户名重复失败")
	}
	if usernameCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被使用，请更换"})
		return
	}

	// 检查邮箱是否已存在
	var emailCount int64
	if err := models.DB.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Count(&emailCount).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"email": email,
			"error": err,
		}).Warn("Register 查询邮箱重复失败")
	}
	if emailCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被注册，请更换"})
		return
	}

	// 验证验证码
	storedCode, err := models.GetVerificationCode(email)
	if err != nil || storedCode != verifyCode {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误或已过期"})
		return
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 创建用户
	user := models.User{
		UserName: username,
		Email:    email,
		Password: string(hashedPassword),
	}

	if result := models.DB.WithContext(ctx).Create(&user); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	// 删除已使用的验证码
	models.DeleteVerificationCode(email)

	// 注册成功后跳转到登录页（使用路由反转）
	c.Redirect(http.StatusFound, routes.Reverse(routes.UserLoginPage))
}

// LoginPage 显示登录页面
// @Summary 登录页面
// @Description 返回用户登录表单页面，支持用户名和邮箱两种登录方式
// @Tags 用户认证
// @Success 200 {string} string "登录页面HTML"
// @Router /user/login [get]
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title":               "用户登录",
		"url_login":           routes.Reverse(routes.UserLoginPage),
		"url_register":        routes.Reverse(routes.UserRegisterPage),
		"url_send_login_code": routes.Reverse(routes.UserSendLoginCode),
		"url_reset_password":  routes.Reverse(routes.UserResetPasswordPage),
	})
}

// SendLoginVerificationCode 发送登录验证码
// @Summary 发送登录验证码
// @Description 根据账号（邮箱或用户名）解析出对应邮箱，发送6位数字登录验证码，5分钟内有效
// @Tags 用户认证
// @Accept x-www-form-urlencoded
// @Produce json
// @Param account formData string true "账号（邮箱或用户名）"
// @Param login_type formData string false "登录类型：email 或 username（默认自动判断）"
// @Success 200 {object} map[string]string "发送成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "账号不存在"
// @Failure 500 {object} map[string]string "发送失败"
// @Router /user/send-login-code [post]
func SendLoginVerificationCode(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	account := c.PostForm("account")
	loginType := c.PostForm("login_type")

	if account == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入账号"})
		return
	}

	// 解析邮箱：用户名登录时通过数据库查找
	var email string
	if loginType == "username" {
		var user models.User
		if err := models.DB.WithContext(ctx).Where("user_name = ?", account).First(&user).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"account": account,
				"type":    "username",
			}).Warn("发送登录验证码失败：用户名不存在")
			c.JSON(http.StatusNotFound, gin.H{"error": "用户名不存在"})
			return
		}
		email = user.Email
	} else {
		// 邮箱登录或自动判断
		if strings.Contains(account, "@") {
			var user models.User
			if err := models.DB.WithContext(ctx).Where("email = ?", account).First(&user).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"account": account,
					"type":    "email",
				}).Warn("发送登录验证码失败：邮箱未注册")
				c.JSON(http.StatusNotFound, gin.H{"error": "该邮箱尚未注册"})
				return
			}
			email = user.Email
		} else {
			// 自动判断为用户名
			var user models.User
			if err := models.DB.WithContext(ctx).Where("user_name = ?", account).First(&user).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"account": account,
					"type":    "username",
				}).Warn("发送登录验证码失败：用户名不存在")
				c.JSON(http.StatusNotFound, gin.H{"error": "用户名不存在"})
				return
			}
			email = user.Email
		}
	}

	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账号未绑定邮箱，无法发送验证码"})
		return
	}

	// 生成6位随机验证码
	code := models.GenerateRandomCode()

	// 存储到Redis（5分钟过期）
	if err := models.SetVerificationCode(email, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码生成失败"})
		return
	}

	// 发送邮件
	if err := models.SendLoginVerificationCode(email, code); err != nil {
		logger.WithFields(map[string]interface{}{
			"to":    email,
			"error": err,
		}).Error("登录验证码邮件发送失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码发送失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送至账号绑定邮箱，请查收"})
}

// Login 处理用户登录（支持用户名或邮箱）
// @Summary 用户登录
// @Description 通过用户名或邮箱进行登录，支持切换登录方式。登录成功后返回Token并设置Cookie。
// @Tags 用户认证
// @Accept x-www-form-urlencoded
// @Produce json
// @Param account formData string true "账号（用户名或邮箱）"
// @Param login_type formData string false "登录类型：email 或 username（默认自动判断）"
// @Param password formData string true "密码"
// @Success 302 {string} string "重定向到首页"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 401 {object} map[string]string "账号或密码错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Security BearerAuth
// @Router /user/login [post]
func Login(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	account := c.PostForm("account")  // 用户名或邮箱
	loginType := c.PostForm("login_type") // "email" 或 "username"
	password := c.PostForm("password")

	if account == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入账号和密码"})
		return
	}

	var user models.User

	// 根据登录类型查询用户
	if loginType == "username" {
		// 用户名登录
		if result := models.DB.WithContext(ctx).Where("user_name = ?", account).First(&user); result.Error != nil {
			logger.WithFields(map[string]interface{}{
				"account": account,
				"type":    "username",
			}).Warn("登录失败：用户名不存在")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名不存在"})
			return
		}
	} else {
		// 默认邮箱登录（也支持自动判断：如果包含@则用邮箱，否则用用户名）
		if strings.Contains(account, "@") {
			if result := models.DB.WithContext(ctx).Where("email = ?", account).First(&user); result.Error != nil {
				logger.WithFields(map[string]interface{}{
					"account": account,
					"type":    "email",
				}).Warn("登录失败：邮箱未注册")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "该邮箱尚未注册"})
				return
			}
		} else {
			if result := models.DB.WithContext(ctx).Where("user_name = ?", account).First(&user); result.Error != nil {
				logger.WithFields(map[string]interface{}{
					"account": account,
					"type":    "username",
				}).Warn("登录失败：用户名不存在")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名不存在"})
				return
			}
		}
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": user.ID,
			"account": account,
		}).Warn("登录失败：密码错误")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	// 校验邮箱验证码
	verifyCode := c.PostForm("verify_code")
	if verifyCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱验证码"})
		return
	}
	storedCode, err := models.GetVerificationCode(user.Email)
	if err != nil || storedCode != verifyCode {
		logger.WithFields(map[string]interface{}{
			"user_id": user.ID,
			"email":   user.Email,
		}).Warn("登录失败：邮箱验证码错误或已过期")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱验证码错误或已过期"})
		return
	}

	// 生成token并存储到Redis
	token := models.GenerateRandomCode() + models.GenerateRandomCode()
	if err := models.SetToken(token, user.ID, user.UserName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败，请重试"})
		return
	}

	// 通过cookie设置token
	c.SetCookie("token", token, 86400, "/", "", false, true)

	// 登录成功后删除已使用的验证码
	models.DeleteVerificationCode(user.Email)

	// 返回 JSON（前端 AJAX 处理），登录成功由前端跳转
	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"msg":      "登录成功",
		"redirect": routes.Reverse(routes.Home),
	})
}

// Logout 用户登出
// @Summary 用户登出
// @Description 清除当前用户的登录Token并重定向到登录页
// @Tags 用户认证
// @Success 302 {string} string "重定向到登录页"
// @Security BearerAuth
// @Router /user/logout [get]
func Logout(c *gin.Context) {
	token, err := c.Cookie("token")
	if err == nil && token != "" {
		models.DeleteToken(token)
	}

	c.SetCookie("token", "", -1, "/", "", false, true)
	// 使用路由反转跳转到登录页
	c.Redirect(http.StatusFound, routes.Reverse(routes.UserLoginPage))
}

// UploadAvatar 上传/更新当前用户头像
// @Summary 上传头像
// @Description 当前登录用户上传头像图片，存储到 MinIO，并更新 users.avatar 字段
// @Tags 用户管理
// @Accept multipart/form-data
// @Produce json
// @Param avatar formData file true "头像图片文件（jpg/png/gif等）"
// @Success 200 {object} map[string]string "上传成功，返回新头像 URL"
// @Failure 400 {object} map[string]string "参数错误（未选择文件/文件过大）"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /user/avatar/upload [post]
func UploadAvatar(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	userID := c.GetUint("user_id")

	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的头像文件"})
		return
	}

	// 限制文件大小（1MB）
	if file.Size > 1*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像文件不能超过 1MB"})
		return
	}

	// 校验文件类型（仅允许图片）
	contentType := file.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true, "image/jpg": true,
		"image/png": true, "image/gif": true, "image/webp": true,
	}
	if !allowedTypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpg/png/gif/webp 格式的图片"})
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Error("UploadAvatar 打开上传文件失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件读取失败"})
		return
	}
	defer src.Close()

	// 上传到 MinIO
	avatarURL, err := models.UploadAvatar(userID, src, file.Filename, contentType)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Error("UploadAvatar 上传到 MinIO 失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "头像上传失败"})
		return
	}

	// 更新数据库
	if err := models.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Update("avatar", avatarURL).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Error("UploadAvatar 更新用户头像字段失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "头像保存失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "头像更新成功",
		"avatar_url": avatarURL,
	})
}

// ResetPasswordPage 显示重置密码页面
// @Summary 重置密码页面
// @Description 返回重置密码表单页面，需输入邮箱、新密码、邮箱验证码
// @Tags 用户认证
// @Success 200 {string} string "重置密码HTML"
// @Router /user/reset-password [get]
func ResetPasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"title":               "重置密码",
		"url_login":           routes.Reverse(routes.UserLoginPage),
		"url_reset_password":  routes.Reverse(routes.UserResetPassword),
		"url_send_reset_code": routes.Reverse(routes.UserSendResetCode),
	})
}

// SendResetVerificationCode 发送重置密码验证码
// @Summary 发送重置密码验证码
// @Description 根据邮箱发送6位数字验证码，5分钟内有效。邮箱必须已注册。
// @Tags 用户认证
// @Accept x-www-form-urlencoded
// @Produce json
// @Param email formData string true "邮箱地址"
// @Success 200 {object} map[string]string "发送成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "邮箱未注册"
// @Failure 500 {object} map[string]string "发送失败"
// @Router /user/send-reset-code [post]
func SendResetVerificationCode(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	email := strings.TrimSpace(c.PostForm("email"))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱地址"})
		return
	}

	// 校验邮箱是否已注册
	var user models.User
	if err := models.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"email": email,
		}).Warn("发送重置密码验证码失败：邮箱未注册")
		c.JSON(http.StatusNotFound, gin.H{"error": "该邮箱尚未注册"})
		return
	}

	// 生成验证码并存 Redis（5分钟过期）
	code := models.GenerateRandomCode()
	if err := models.SetVerificationCode(email, code); err != nil {
		logger.WithFields(map[string]interface{}{
			"email": email,
			"error": err,
		}).Error("SendResetVerificationCode 存储验证码失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码生成失败"})
		return
	}

	// 发送邮件
	if err := models.SendResetPasswordVerificationCode(email, code); err != nil {
		logger.WithFields(map[string]interface{}{
			"to":    email,
			"error": err,
		}).Error("重置密码验证码邮件发送失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码发送失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送至邮箱，请查收"})
}

// ResetPassword 重置密码（需邮箱 + 邮箱验证码 + 新密码）
// @Summary 重置密码
// @Description 通过邮箱验证码重置密码，无需登录态
// @Tags 用户认证
// @Accept x-www-form-urlencoded
// @Produce json
// @Param email formData string true "邮箱地址"
// @Param new_password formData string true "新密码（至少6位）"
// @Param verify_code formData string true "邮箱验证码"
// @Success 200 {object} map[string]string "重置成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 401 {object} map[string]string "验证码错误或已过期"
// @Failure 404 {object} map[string]string "邮箱未注册"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /user/reset-password [post]
func ResetPassword(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	email := strings.TrimSpace(c.PostForm("email"))
	newPassword := c.PostForm("new_password")
	verifyCode := c.PostForm("verify_code")

	if email == "" || newPassword == "" || verifyCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱、新密码、验证码均不能为空"})
		return
	}
	if len(newPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度至少6位"})
		return
	}

	// 校验邮箱是否已注册
	var user models.User
	if err := models.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "该邮箱尚未注册"})
		return
	}

	// 校验验证码
	storedCode, err := models.GetVerificationCode(email)
	if err != nil || storedCode != verifyCode {
		logger.WithFields(map[string]interface{}{
			"email": email,
		}).Warn("重置密码失败：邮箱验证码错误或已过期")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱验证码错误或已过期"})
		return
	}

	// 加密新密码
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"email": email,
			"error": err,
		}).Error("ResetPassword 密码加密失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 更新密码
	if err := models.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).
		Update("password", string(hashed)).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": user.ID,
			"error":   err,
		}).Error("ResetPassword 更新密码失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码重置失败"})
		return
	}

	// 删除已用验证码
	models.DeleteVerificationCode(email)

	c.JSON(http.StatusOK, gin.H{
		"message":  "密码重置成功",
		"redirect": routes.Reverse(routes.UserLoginPage),
	})
}
