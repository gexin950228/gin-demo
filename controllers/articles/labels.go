package articles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"gin-demo/models"
)

// ListLabels handles GET /articles/labels
func ListLabels(c *gin.Context) {
	ls, err := models.ListLabels()
	if err != nil {
		logrus.Errorf("labels: list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	res := make([]string, 0, len(ls))
	for _, l := range ls {
		res = append(res, l.Name)
	}
	c.JSON(http.StatusOK, gin.H{"labels": res})
}
