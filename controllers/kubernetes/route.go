package kubernetes

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all kubernetes-related routes onto the provided RouterGroup.
func RegisterRoutes(k8s *gin.RouterGroup) {
	k8s.GET("/namespaces", GetNamespaces)
	k8s.GET("/deployments", GetDeployments)
	k8s.GET("/daemonsets", GetDaemonSets)
	k8s.GET("/statefulsets", GetStatefulSets)
	k8s.GET("/jobs", GetJobs)
	k8s.GET("/cronjobs", GetCronJobs)
	k8s.GET("/services", GetServices)
	k8s.GET("/deployments/pods", GetPodsForDeployment)
	k8s.GET("/daemonsets/pods", GetPodsForDaemonSet)
	k8s.GET("/statefulsets/pods", GetPodsForStatefulSet)
	k8s.GET("/deployments/yaml", GetDeploymentYAML)
	k8s.POST("/deployments/update", UpdateDeployment)
	k8s.GET("/daemonsets/yaml", GetDaemonSetYAML)
	k8s.POST("/daemonsets/update", UpdateDaemonSet)
	k8s.GET("/statefulsets/yaml", GetStatefulSetYAML)
	k8s.POST("/statefulsets/update", UpdateStatefulSet)
	k8s.GET("/jobs/yaml", GetJobYAML)
	k8s.POST("/jobs/update", UpdateJob)
	k8s.GET("/cronjobs/yaml", GetCronJobYAML)
	k8s.POST("/cronjobs/update", UpdateCronJob)
	k8s.GET("/services/yaml", GetServiceYAML)
	k8s.POST("/services/update", UpdateService)
}