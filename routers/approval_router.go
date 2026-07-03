package routers

import (
	approvalhdl "gin-demo/handlers/approval"
	"github.com/gin-gonic/gin"
)

// SetupApprovalRoutes 设置审批相关路由
func SetupApprovalRoutes(r *gin.RouterGroup) {
	approval := r.Group("/approvals")
	{
		approval.GET("/list", approvalhdl.ApprovalListPage)
		approval.GET("/detail", approvalhdl.ApprovalDetailAPI)
		approval.GET("/pending-count", approvalhdl.PendingApprovalCountAPI)
		approval.POST("/approve", approvalhdl.ApproveArticle)
		approval.POST("/reject", approvalhdl.RejectArticle)
		approval.POST("/revoke", approvalhdl.RevokeApproval)
	}
}
