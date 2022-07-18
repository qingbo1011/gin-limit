package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Unlimited(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"msg": "没有使用限流",
	})
}

func LimitedByRateLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"msg": "中间件1：使用RateLimit进行限流",
	})
}

func LimitedByMaxAllowed(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"msg": "中间件2：使用MaxAllowed进行限流",
	})
}

func LimitedByNewRateLimiter(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"msg": "中间件3：使用NewRateLimiter进行限流",
	})
}

func LimitedByThrottle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"msg": "中间件4：使用Throttle进行限流",
	})
}
