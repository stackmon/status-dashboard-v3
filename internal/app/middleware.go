package app

import (
	"github.com/gin-gonic/gin"
)

func ErrorHandle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		err := c.Errors.Last()
		if err == nil {
			return
		}

		c.JSON(-1, gin.H{
			"errorMsg": err.Error(),
		})
	}
}

func Return404(c *gin.Context) {
	c.JSON(404, gin.H{"errorMsg": "page not found"})
}
