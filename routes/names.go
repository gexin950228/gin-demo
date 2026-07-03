package routes

import (
	"fmt"
	"strings"
)

// 路由名称常量定义
const (
	// 用户相关路由
	UserRegisterPage   = "user.register.page"
	UserSendCode       = "user.send.code"
	UserSendLoginCode  = "user.send.login.code"
	UserSendResetCode  = "user.send.reset.code"
	UserResetPasswordPage = "user.reset.password.page"
	UserResetPassword  = "user.reset.password"
	UserAvatarUpload   = "user.avatar.upload"
	UserRegister       = "user.register"
	UserLoginPage      = "user.login.page"
	UserLogin          = "user.login"
	UserLogout         = "user.logout"
	UserCheckUsername  = "user.check.username"
	UserCheckEmail     = "user.check.email"

	// 首页
	Home = "home"

	// 文章相关路由
	ArticleListPage   = "article.list.page"
	ArticleListAPI    = "article.list.api"
	ArticleCreatePage = "article.create.page"
	ArticleCreate     = "article.create"
	ArticleView       = "article.view"
	ArticleEditPage   = "article.edit.page"
	ArticleDelete     = "article.delete"

	// 评论相关路由
	CommentList         = "article.comment.list"
	CommentCreate       = "article.comment.create"
	CommentDelete       = "article.comment.delete"
	CommentImageUpload  = "article.comment.image.upload"

	// 消息通知相关路由
	NotificationListPage    = "article.notification.list.page"
	NotificationList        = "article.notification.list"
	NotificationUnreadCount = "article.notification.unread.count"
	NotificationRead        = "article.notification.read"

	// 标签相关路由
	TagManagePage     = "tag.manage.page"

	// 审批相关路由
	ApprovalListPage   = "approval.list.page"
	ApprovalDetailAPI  = "approval.detail.api"
	ApprovalApprove    = "approval.approve"
	ApprovalReject     = "approval.reject"
	ApprovalRevoke     = "approval.revoke"
	ApprovalPendingCount = "approval.pending.count"
)

// 路由URL映射表（唯一数据源）
var RouteMap = map[string]string{
	// 用户相关
	UserRegisterPage:      "/user/register",
	UserSendCode:          "/user/send-code",
	UserSendLoginCode:     "/user/send-login-code",
	UserSendResetCode:     "/user/send-reset-code",
	UserResetPasswordPage: "/user/reset-password",
	UserResetPassword:     "/user/reset-password",
	UserAvatarUpload:      "/user/avatar/upload",
	UserRegister:          "/user/register",
	UserLoginPage:         "/user/login",
	UserLogin:             "/user/login",
	UserLogout:            "/user/logout",
	UserCheckUsername:     "/user/check-username",
	UserCheckEmail:        "/user/check-email",

	// 首页
	Home: "/home",

	// 文章相关
	ArticleListPage:   "/articles/list",
	ArticleListAPI:    "/articles/api/list",
	ArticleCreatePage: "/articles/create",
	ArticleCreate:     "/articles/create",
	ArticleView:       "/articles/view",
	ArticleEditPage:   "/articles/edit",
	ArticleDelete:     "/articles/delete",

	// 评论相关
	CommentList:        "/articles/api/comments",
	CommentCreate:      "/articles/api/comments/create",
	CommentDelete:      "/articles/api/comments/delete",
	CommentImageUpload: "/articles/api/comments/upload-image",

	// 消息通知相关
	NotificationListPage:    "/notifications/list",
	NotificationList:        "/articles/api/notifications",
	NotificationUnreadCount: "/articles/api/notifications/unread-count",
	NotificationRead:        "/articles/api/notifications/read",

	// 标签相关
	TagManagePage:     "/tags/manage",

	// 审批相关
	ApprovalListPage:     "/approvals/list",
	ApprovalDetailAPI:    "/approvals/detail",
	ApprovalApprove:      "/approvals/approve",
	ApprovalReject:       "/approvals/reject",
	ApprovalRevoke:       "/approvals/revoke",
	ApprovalPendingCount: "/approvals/pending-count",
}

// Reverse 根据路由名称获取URL（支持参数替换）
// 示例:
//
//	routes.Reverse(routes.Home)                          => "/home"
//	routes.Reverse(routes.ArticleView, "id", "123")      => "/articles/view?id=123"
func Reverse(routeName string, params ...interface{}) string {
	url, ok := RouteMap[routeName]
	if !ok {
		return "/" // 找不到返回根路径
	}

	var queryParams []string

	// 如果有参数，按顺序处理
	if len(params) > 0 && len(params)%2 == 0 {
		for i := 0; i < len(params); i += 2 {
			paramName := params[i].(string)
			paramValue := fmt.Sprintf("%v", params[i+1])

			// 尝试替换路径中的 :param 占位符
			replaced := strings.ReplaceAll(url, ":"+paramName, paramValue)
			if replaced == url {
				// 路径中无此占位符，作为查询参数追加
				queryParams = append(queryParams, paramName+"="+paramValue)
			} else {
				url = replaced
			}
		}
	}

	// 追加查询参数
	if len(queryParams) > 0 {
		url = url + "?" + strings.Join(queryParams, "&")
	}

	return url
}
