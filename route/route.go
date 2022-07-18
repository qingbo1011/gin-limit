package route

import (
	"gin-limited/middreware"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// NewRouter 路由配置
func NewRouter() *gin.Engine {
	r := gin.Default()

	unlimited := r.Group("/unlimited")
	{
		unlimited.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"msg": "没有使用限流",
			})
		})

	}

	limit1 := r.Group("/limit1").Use(middreware.RateLimit())
	{
		limit1.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"msg": "中间件1：使用RateLimit进行限流",
			})
		})
	}

	limit2 := r.Group("/limit2").Use(middreware.MaxAllowed(1000))
	{
		limit2.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"msg": "中间件2：使用MaxAllowed进行限流",
			})
		})
	}

	limit3 := r.Group("/limit3").Use(middreware.NewRateLimiter(func(c *gin.Context) string {
		return c.ClientIP() // limit rate by client ip
	}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
		// limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
		return rate.NewLimiter(rate.Every(100*time.Millisecond), 10), time.Hour
	}, func(c *gin.Context) {
		c.AbortWithStatus(429) // handle exceed rate limit request
		return
	}))
	{
		limit3.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"msg": "中间件3：使用NewRateLimiter进行限流",
			})
		})
	}

	limit4 := r.Group("/limit4").Use(middreware.Throttle(1000, 20))
	{
		limit4.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"msg": "中间件4：使用Throttle进行限流",
			})
		})
	}

	return r
}
