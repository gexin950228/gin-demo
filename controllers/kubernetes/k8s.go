package kubernetes

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset

func init() {
	// Load kubeconfig
	kubeconfig := filepath.Join("conf", "kubernetes.config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// Create clientset
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

// GetNamespaces returns list of namespaces
func GetNamespaces(c *gin.Context) {
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var nsList []string
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.Name)
	}

	c.JSON(http.StatusOK, gin.H{"namespaces": nsList})
}

// GetDeployments returns deployments for a namespace
func GetDeployments(c *gin.Context) {
	namespace := c.Query("ns")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace required"})
		return
	}

	deployments, err := clientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deployments": deployments.Items})
}

// GetDaemonSets returns daemonsets for a namespace
func GetDaemonSets(c *gin.Context) {
	namespace := c.Query("ns")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace required"})
		return
	}

	daemonsets, err := clientset.AppsV1().DaemonSets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"daemonsets": daemonsets.Items})
}

// GetStatefulSets returns statefulsets for a namespace
func GetStatefulSets(c *gin.Context) {
	namespace := c.Query("ns")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace required"})
		return
	}

	statefulsets, err := clientset.AppsV1().StatefulSets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"statefulsets": statefulsets.Items})
}

// GetJobs returns jobs for a namespace
func GetJobs(c *gin.Context) {
	namespace := c.Query("ns")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace required"})
		return
	}

	jobs, err := clientset.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs.Items})
}

// GetCronJobs returns cronjobs for a namespace
func GetCronJobs(c *gin.Context) {
	namespace := c.Query("ns")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace required"})
		return
	}

	cronjobs, err := clientset.BatchV1().CronJobs(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cronjobs": cronjobs.Items})
}

// GetPodsForDeployment returns pods controlled by a deployment
func GetPodsForDeployment(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	selector := deployment.Spec.Selector.MatchLabels
	labelSelector := ""
	for k, v := range selector {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += k + "=" + v
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pods": pods.Items})
}

// GetPodsForDaemonSet returns pods controlled by a daemonset
func GetPodsForDaemonSet(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	selector := ds.Spec.Selector.MatchLabels
	labelSelector := ""
	for k, v := range selector {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += k + "=" + v
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pods": pods.Items})
}

// GetPodsForStatefulSet returns pods controlled by a statefulset
func GetPodsForStatefulSet(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	sts, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	selector := sts.Spec.Selector.MatchLabels
	labelSelector := ""
	for k, v := range selector {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += k + "=" + v
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pods": pods.Items})
}

// GetServices returns services for a namespace
func GetServices(c *gin.Context) {
	namespace := c.Query("ns")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace required"})
		return
	}

	services, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"services": services.Items})
}

// GetDeploymentYAML returns YAML of a deployment
func GetDeploymentYAML(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	dep, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlData, err := yaml.Marshal(dep)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(yamlData))
}

// UpdateDeployment updates a deployment from YAML
func UpdateDeployment(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	yamlStr := c.PostForm("yaml")
	if namespace == "" || name == "" || yamlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, name and yaml required"})
		return
	}

	var dep appsv1.Deployment
	err := yaml.Unmarshal([]byte(yamlStr), &dep)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), &dep, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// GetDaemonSetYAML returns YAML of a daemonset
func GetDaemonSetYAML(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlData, err := yaml.Marshal(ds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(yamlData))
}

// UpdateDaemonSet updates a daemonset from YAML
func UpdateDaemonSet(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	yamlStr := c.PostForm("yaml")
	if namespace == "" || name == "" || yamlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, name and yaml required"})
		return
	}

	var ds appsv1.DaemonSet
	err := yaml.Unmarshal([]byte(yamlStr), &ds)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = clientset.AppsV1().DaemonSets(namespace).Update(context.TODO(), &ds, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// GetStatefulSetYAML returns YAML of a statefulset
func GetStatefulSetYAML(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	sts, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlData, err := yaml.Marshal(sts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(yamlData))
}

// UpdateStatefulSet updates a statefulset from YAML
func UpdateStatefulSet(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	yamlStr := c.PostForm("yaml")
	if namespace == "" || name == "" || yamlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, name and yaml required"})
		return
	}

	var sts appsv1.StatefulSet
	err := yaml.Unmarshal([]byte(yamlStr), &sts)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = clientset.AppsV1().StatefulSets(namespace).Update(context.TODO(), &sts, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// GetJobYAML returns YAML of a job
func GetJobYAML(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	job, err := clientset.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlData, err := yaml.Marshal(job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(yamlData))
}

// UpdateJob updates a job from YAML
func UpdateJob(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	yamlStr := c.PostForm("yaml")
	if namespace == "" || name == "" || yamlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, name and yaml required"})
		return
	}

	var job batchv1.Job
	err := yaml.Unmarshal([]byte(yamlStr), &job)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = clientset.BatchV1().Jobs(namespace).Update(context.TODO(), &job, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// GetCronJobYAML returns YAML of a cronjob
func GetCronJobYAML(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	cj, err := clientset.BatchV1().CronJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlData, err := yaml.Marshal(cj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(yamlData))
}

// UpdateCronJob updates a cronjob from YAML
func UpdateCronJob(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	yamlStr := c.PostForm("yaml")
	if namespace == "" || name == "" || yamlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, name and yaml required"})
		return
	}

	var cj batchv1.CronJob
	err := yaml.Unmarshal([]byte(yamlStr), &cj)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = clientset.BatchV1().CronJobs(namespace).Update(context.TODO(), &cj, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// GetServiceYAML returns YAML of a service
func GetServiceYAML(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name required"})
		return
	}

	svc, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlData, err := yaml.Marshal(svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(yamlData))
}

// UpdateService updates a service from YAML
func UpdateService(c *gin.Context) {
	namespace := c.Query("ns")
	name := c.Query("name")
	yamlStr := c.PostForm("yaml")
	if namespace == "" || name == "" || yamlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, name and yaml required"})
		return
	}

	var svc corev1.Service
	err := yaml.Unmarshal([]byte(yamlStr), &svc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = clientset.CoreV1().Services(namespace).Update(context.TODO(), &svc, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}
