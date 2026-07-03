package user

import (
	"regexp"
	"strings"
	"unicode"
)

// ValidateResult 验证结果
type ValidateResult struct {
	Valid   bool
	Message string
}

// ValidateUsernameFormat 验证用户名格式
func ValidateUsernameFormat(username string) ValidateResult {
	// 检查长度
	if len(username) < 3 {
		return ValidateResult{false, "用户名至少需要3个字符"}
	}
	if len(username) > 20 {
		return ValidateResult{false, "用户名不能超过20个字符"}
	}

	// 检查是否只包含合法字符：字母、数字、下划线、中文
	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' &&
			!(r >= 0x4e00 && r <= 0x9fa5) { // 中文范围
			return ValidateResult{false, "用户名只能包含字母、数字、下划线和中文"}
		}
	}

	return ValidateResult{true, ""}
}

// ValidateEmailFormat 严格验证邮箱格式
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ValidateEmailFormat(email string) ValidateResult {
	email = strings.TrimSpace(email)

	if email == "" {
		return ValidateResult{false, "邮箱地址不能为空"}
	}

	// 基本格式检查
	if !emailRegex.MatchString(email) {
		return ValidateResult{false, "邮箱格式不正确，请输入有效邮箱（如 example@163.com）"}
	}

	// 检查连续点
	if strings.Contains(email, "..") {
		return ValidateResult{false, "邮箱地址不能包含连续的点号"}
	}

	// 检查@符号数量
	if strings.Count(email, "@") != 1 {
		return ValidateResult{false, "邮箱地址必须包含且仅包含一个@"}
	}

	// 分割邮箱
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ValidateResult{false, "邮箱格式不正确"}
	}

	localPart := parts[0]
	domainPart := parts[1]

	// 本地部分不能以点开头或结尾
	if len(localPart) > 0 && (localPart[0] == '.' || localPart[len(localPart)-1] == '.') {
		return ValidateResult{false, "邮箱格式不正确"}
	}

	// 域名部分检查
	if domainPart == "" || !strings.Contains(domainPart, ".") {
		return ValidateResult{false, "邮箱域名格式不正确"}
	}

	// 检查域名中的连续点
	if strings.Contains(domainPart, "..") {
		return ValidateResult{false, "邮箱域名格式不正确"}
	}

	// TLD至少2个字符
	domainParts := strings.Split(domainPart, ".")
	tld := domainParts[len(domainParts)-1]
	if len(tld) < 2 {
		return ValidateResult{false, "邮箱域名后缀不正确"}
	}

	return ValidateResult{true, ""}
}

// PasswordStrength 密码强度信息
type PasswordStrength struct {
	Level    string `json:"level"`     // weak, medium, strong
	Score    int    `json:"score"`     // 0-5
	Rules    map[string]bool `json:"rules"` // 各项规则通过情况
	Message  string `json:"message"`
}

// ValidatePasswordStrength 验证密码强度
func ValidatePasswordStrength(password string) PasswordStrength {
	rules := make(map[string]bool)

	// 规则1: 长度至少8位
	rules["length"] = len(password) >= 8

	// 规则2: 包含大写字母
	rules["upper"] = false
	for _, r := range password {
		if unicode.IsUpper(r) {
			rules["upper"] = true
			break
		}
	}

	// 规则3: 包含小写字母
	rules["lower"] = false
	for _, r := range password {
		if unicode.IsLower(r) {
			rules["lower"] = true
			break
		}
	}

	// 规则4: 包含数字
	rules["number"] = false
	for _, r := range password {
		if unicode.IsDigit(r) {
			rules["number"] = true
			break
		}
	}

	// 规则5: 包含特殊字符
	specialChars := "!@#$%^&*(),.?\":{}|<>"
	rules["special"] = false
	for _, r := range password {
		if strings.ContainsRune(specialChars, r) {
			rules["special"] = true
			break
		}
	}

	// 计算得分
	score := 0
	if rules["length"] { score++ }
	if rules["upper"] { score++ }
	if rules["lower"] { score++ }
	if rules["number"] { score++ }
	if rules["special"] { score++ }

	// 判断等级和提示信息
	var level, message string
	switch {
	case score <= 2:
		level = "weak"
		message = "弱 - 建议使用大小写字母+数字+特殊字符的组合，至少8位"
	case score <= 3:
		level = "medium"
		message = "中等 - 建议添加更多字符类型以提高安全性"
	default:
		level = "strong"
		message = "强 - 密码安全性良好"
	}

	return PasswordStrength{
		Level:   level,
		Score:   score,
		Rules:   rules,
		Message: message,
	}
}
