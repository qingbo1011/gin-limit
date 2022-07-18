package middreware

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
	"github.com/patrickmn/go-cache"
	"golang.org/x/time/rate"
)

func RateLimit() gin.HandlerFunc {
	bucket := ratelimit.NewBucket(time.Microsecond*100, int64(10000)) // 每100ms填充一个令牌，令牌桶容量为10000
	return func(c *gin.Context) {
		// 如果取不到令牌就中断本次请求返回 rate limit...
		if bucket.TakeAvailable(1) < 1 {
			c.JSON(http.StatusOK, gin.H{
				"msg": "rate limit...",
			})
			c.Abort()
			return
		}
	}
}

func MaxAllowed(n int) gin.HandlerFunc {
	sem := make(chan struct{}, n)
	acquire := func() { sem <- struct{}{} }
	release := func() { <-sem }
	return func(c *gin.Context) {
		acquire()       // before request
		defer release() // after request
		c.Next()
	}
}

var limiterSet = cache.New(5*time.Minute, 10*time.Minute)

func NewRateLimiter(key func(*gin.Context) string, createLimiter func(*gin.Context) (*rate.Limiter, time.Duration),
	abort func(*gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		k := key(c)
		limiter, ok := limiterSet.Get(k)
		if !ok {
			var expire time.Duration
			limiter, expire = createLimiter(c)
			limiterSet.Set(k, limiter, expire)
		}
		ok = limiter.(*rate.Limiter).Allow()
		if !ok {
			abort(c)
			return
		}
	}
}

// Throttle used to check the rate limit of incoming request
func Throttle(maxEventsPerSec int, maxBurstSize int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(maxEventsPerSec), maxBurstSize)

	return func(context *gin.Context) {
		if limiter.Allow() {
			context.Next()
			return
		}
		context.Error(errors.New("Limit exceeded"))
		context.AbortWithStatus(http.StatusTooManyRequests)
	}
}
