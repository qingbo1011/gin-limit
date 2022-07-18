package route

import (
	"gin-limited/api"
	"gin-limited/middreware"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// NewRouter 路由配置
func NewRouter() *gin.Engine {
	r := gin.Default()

	unlimited := r.Group("/unlimited")
	{
		unlimited.GET("/", api.Unlimited)
	}

	limit1 := r.Group("/limit1").Use(middreware.RateLimit())
	{
		limit1.GET("/", api.LimitedByRateLimit)
	}

	limit2 := r.Group("/limit2").Use(middreware.MaxAllowed(1000))
	{
		limit2.GET("/", api.LimitedByMaxAllowed)
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
		limit3.GET("/", api.LimitedByNewRateLimiter)
	}

	limit4 := r.Group("/limit4").Use(middreware.Throttle(1000, 20))
	{
		limit4.GET("/", api.LimitedByThrottle)
	}

	return r
}
