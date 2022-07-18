# Gin限流

参考李文周老师的博客：[常用限流策略——漏桶与令牌桶介绍](https://www.liwenzhou.com/posts/Go/ratelimit/)

# 限流

限流又称为流量控制（流控），通常是指限制到达系统的并发请求数。

> 我们生活中也会经常遇到限流的场景，比如：某景区限制每日进入景区的游客数量为8万人；沙河地铁站早高峰通过站外排队逐一放行的方式限制同一时间进入车站的旅客数量等。

限流虽然会影响部分用户的使用体验，但是却能在一定程度上报障系统的稳定性，不至于崩溃（大家都没了用户体验）。

而互联网上类似需要限流的业务场景也有很多，比如电商系统的秒杀、微博上突发热点新闻、双十一购物节、12306抢票等等。这些场景下的用户请求量通常会激增，远远超过平时正常的请求量，此时如果不加任何限制很容易就会将后端服务打垮，影响服务的稳定性。

> 此外，一些厂商公开的API服务通常也会限制用户的请求次数，比如百度地图开放平台等会根据用户的付费情况来限制用户的请求数等。
>
> ![](https://img-qingbo.oss-cn-beijing.aliyuncs.com/img/20220712151400.png)

# 常用的限流策略

## 漏桶

漏桶(Leaky bucket)限流很好理解，假设我们有一个水桶按固定的速率向下方滴落一滴水，无论有多少请求，请求的速率有多大，都按照固定的速率流出，对应到系统中就是按照固定的速率处理请求。

![](https://img-qingbo.oss-cn-beijing.aliyuncs.com/img/20220712151701.png)

漏桶法的关键点在于漏桶**始终按照固定的速率运行**，但是它**并不能很好的处理有大量突发请求的场景**，毕竟在某些场景下我们可能需要提高系统的处理效率，而不是一味的按照固定速率处理请求。

关于漏桶的实现，uber团队有一个开源的[github.com/uber-go/ratelimit](https://github.com/uber-go/ratelimit)库。 这个库的使用方法比较简单，`Take()` 方法会返回漏桶下一次滴水的时间。

代码如下：

```go
package main

import (
   "fmt"
   "time"

   "go.uber.org/ratelimit"
)

func main() {
   r1 := ratelimit.New(10) // (10表示1s/10，即每100ms(0.1s)运行一次；同理，1表示1s/1,即每秒运行一次)
   prev := time.Now()
   for i := 0; i < 10; i++ {
      now := r1.Take()
      fmt.Println(i, now.Sub(prev))
      prev = now
   }
}
```

输出结果：

![](https://img-qingbo.oss-cn-beijing.aliyuncs.com/img/20220712154826.png)

> 注意：在Windows下，如果rate设置为100，即`r1 := ratelimit.New(100)`（亲测100有时有效有时无效），可能会失效
>
> 在README文件中作者已经说了：
>
> - Why does example_test.go fail when I run it locally on Windows? (based on #80)
>
>   Windows has some known issues with timers precision. See [golang/go#44343](https://github.com/golang/go/issues/44343). We don't expect to work around it.

[github.com/uber-go/ratelimit](https://github.com/uber-go/ratelimit)库的源码实现也比较简单，这里大致说一下关键的地方，具体可以去github上看一下完整代码。（或者直接在goland中Ctrl 点击进入源码，这样更好看一点）

> 限制器是一个接口类型，其要求实现一个`Take()`方法：
>
> ```go
> // Limiter is used to rate-limit some process, possibly across goroutines.
> // The process is expected to call Take() before every iteration, which
> // may block to throttle the goroutine.
> type Limiter interface {
> // Take should block to make sure that the RPS is met.
> // 翻译：Take方法应该阻塞已确保满足 RPS
> Take() time.Time
> }
> ```
>
> 实现限制器接口的结构体定义如下：
>
> ```go
> type limiter struct {
> 	sync.Mutex                // 锁
> 	last       time.Time      // 上一次的时刻
> 	sleepFor   time.Duration  // 需要等待的时间
> 	perRequest time.Duration  // 每次的时间间隔
> 	maxSlack   time.Duration  // 最大的富余量
> 	clock      Clock          // 时钟
> }
> ```
>
> 这里可以重点留意下`maxSlack`字段，它在后面的`Take()`方法中的处理。
>
> `limiter`结构体实现`Limiter`接口的`Take()`方法内容如下：
>
> ```go
> // Take 会阻塞确保两次请求之间的时间走完
> // Take 调用平均数为 time.Second/rate.
> func (t *limiter) Take() time.Time {
> 	t.Lock()
> 	defer t.Unlock()
> 
> 	now := t.clock.Now()
> 
> 	// 如果是第一次请求就直接放行
> 	if t.last.IsZero() {
> 		t.last = now
> 		return t.last
> 	}
> 
> 	// sleepFor 根据 perRequest 和上一次请求的时刻计算应该sleep的时间
> 	// 由于每次请求间隔的时间可能会超过perRequest, 所以这个数字可能为负数，并在多个请求之间累加
> 	t.sleepFor += t.perRequest - now.Sub(t.last)
> 
> 	// 我们不应该让sleepFor负的太多，因为这意味着一个服务在短时间内慢了很多随后会得到更高的RPS。
> 	if t.sleepFor < t.maxSlack {
> 		t.sleepFor = t.maxSlack
> 	}
> 
> 	// 如果 sleepFor 是正值那么就 sleep
> 	if t.sleepFor > 0 {
> 		t.clock.Sleep(t.sleepFor)
> 		t.last = now.Add(t.sleepFor)
> 		t.sleepFor = 0
> 	} else {
> 		t.last = now
> 	}
> 
> 	return t.last
> }
> ```
>
> 上面的代码根据记录每次请求的间隔时间和上一次请求的时刻来计算当次请求需要阻塞的时间——`sleepFor`，这里需要留意的是`sleepFor`的值可能为负，在经过间隔时间长的两次访问之后会导致随后大量的请求被放行，所以代码中针对这个场景有专门的优化处理。创建限制器的`New()`函数中会为`maxSlack`设置初始值，也可以通过`WithoutSlack`这个Option取消这个默认值。
>
> ```go
> func New(rate int, opts ...Option) Limiter {
> 	l := &limiter{
> 		perRequest: time.Second / time.Duration(rate),
> 		maxSlack:   -10 * time.Second / time.Duration(rate),
> 	}
> 	for _, opt := range opts {
> 		opt(l)
> 	}
> 	if l.clock == nil {
> 		l.clock = clock.New()
> 	}
> 	return l
> }
> ```

## 令牌桶

令牌桶(Token bucket)其实和漏桶的原理类似，**令牌桶按固定的速率往桶里放入令牌，并且只要能从桶里取出令牌就能通过**，**令牌桶支持突发流量的快速处理**。

![](https://img-qingbo.oss-cn-beijing.aliyuncs.com/img/20220712162624.png)

对于从桶里取不到令牌的场景，我们可以选择等待也可以直接拒绝并返回。

对于令牌桶的Go语言实现，大家可以参照[github.com/juju/ratelimit](https://github.com/juju/ratelimit)库。这个库支持多种令牌桶模式，并且使用起来也比较简单。

创建令牌桶的方法：

```go
// 创建指定填充速率和容量大小的令牌桶
func NewBucket(fillInterval time.Duration, capacity int64) *Bucket
// 创建指定填充速率、容量大小和每次填充的令牌数的令牌桶
func NewBucketWithQuantum(fillInterval time.Duration, capacity, quantum int64) *Bucket
// 创建填充速度为指定速率和容量大小的令牌桶
// NewBucketWithRate(0.1, 200) 表示每秒填充20个令牌
func NewBucketWithRate(rate float64, capacity int64) *Bucket
```

取出令牌的方法如下：

```go
// 取token（非阻塞）
func (tb *Bucket) Take(count int64) time.Duration
func (tb *Bucket) TakeAvailable(count int64) int64

// 最多等maxWait时间取token
func (tb *Bucket) TakeMaxDuration(count int64, maxWait time.Duration) (time.Duration, bool)

// 取token（阻塞）
func (tb *Bucket) Wait(count int64)
func (tb *Bucket) WaitMaxDuration(count int64, maxWait time.Duration) bool
```

虽说是令牌桶，但是我们没有必要真的去生成令牌放到桶里，我们只需要每次来取令牌的时候计算一下，当前是否有足够的令牌就可以了，具体的计算方式可以总结为下面的公式：

`当前令牌数 = 上一次剩余的令牌数 + (本次取令牌的时刻-上一次取令牌的时刻)/放置令牌的时间间隔 * 每次放置的令牌数`

> [github.com/juju/ratelimit](https://github.com/juju/ratelimit)这个库中关于令牌数计算的源代码如下：
>
> ```go
> func (tb *Bucket) currentTick(now time.Time) int64 {
> 	return int64(now.Sub(tb.startTime) / tb.fillInterval)
> }
> func (tb *Bucket) adjustavailableTokens(tick int64) {
> 	if tb.availableTokens >= tb.capacity {
> 		return
> 	}
> 	tb.availableTokens += (tick - tb.latestTick) * tb.quantum
> 	if tb.availableTokens > tb.capacity {
> 		tb.availableTokens = tb.capacity
> 	}
> 	tb.latestTick = tick
> 	return
> }
> ```
>
> 获取令牌的`TakeAvailable()`函数关键部分的源代码如下：
>
> ```go
> func (tb *Bucket) takeAvailable(now time.Time, count int64) int64 {
> 	if count <= 0 {
> 		return 0
> 	}
> 	tb.adjustavailableTokens(tb.currentTick(now))
> 	if tb.availableTokens <= 0 {
> 		return 0
> 	}
> 	if count > tb.availableTokens {
> 		count = tb.availableTokens
> 	}
> 	tb.availableTokens -= count
> 	return count
> }
> ```
>
> 从代码中也可以看到其实令牌桶的实现并没有很复杂。

# gin中使用限流中间件

在gin框架构建的项目中，我们可以将限流组件定义成中间件。

## 自定义中间件

这里使用**令牌桶**作为限流策略，编写一个限流中间件如下：

```go
package middreware

import (
   "net/http"
   "time"

   "github.com/gin-gonic/gin"
   "github.com/juju/ratelimit"
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
```

## 开源限流中间件

- https://github.com/aviddiviner/gin-limit

  ```go
  package limit
  
  import (
  	"github.com/gin-gonic/gin"
  )
  
  func MaxAllowed(n int) gin.HandlerFunc {
  	sem := make(chan struct{}, n)
  	acquire := func() { sem <- struct{}{} }
  	release := func() { <-sem }
  	return func(c *gin.Context) {
  		acquire() // before request
  		defer release() // after request
  		c.Next()
  	}
  }
  ```

- https://github.com/yangxikun/gin-limit-by-key

  ```go
  package limit
  
  import (
  	"time"
  
  	"github.com/gin-gonic/gin"
  	"github.com/patrickmn/go-cache"
  	"golang.org/x/time/rate"
  )
  
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
  		c.Next()
  	}
  }
  ```

- https://github.com/takeshiyu/gin-throttle

  ```go
  package middleware
  
  import (
  	"errors"
  	"net/http"
  
  	"github.com/gin-gonic/gin"
  	"golang.org/x/time/rate"
  )
  
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
  ```

……

网上关于gin限流中间件有很多优秀的开源代码。其中有些是结合redis的。以后可以按照需求，去继续学习使用。

## 关于gin中间件中c.Next()的思考

在整理限流中间件时，发现它们几乎都在最后使用了`c.Next()`，而我理解的`c.Next()`，就是在中间件执行完后续的一些事情（[Next()方法](https://www.topgoer.com/gin%E6%A1%86%E6%9E%B6/gin%E4%B8%AD%E9%97%B4%E4%BB%B6/next%E6%96%B9%E6%B3%95.html)）。但是上面的那些限流中间件，在`c.Next()`之后，也没有写后续要执行的代码啊？这就让我对`c.Next()`的用法产生了疑惑。

后来参考这篇文章：**[Gin 中间件Next()方法作用解析](https://blog.dianduidian.com/post/gin-%E4%B8%AD%E9%97%B4%E4%BB%B6next%E6%96%B9%E6%B3%95%E5%8E%9F%E7%90%86%E8%A7%A3%E6%9E%90/)**，算是搞明白了。这里做一下搬运，记录一下：

### 背景

> 关于`Gin`中间件`Context.Next()`的用途我之前一直认为就是用在当前中间件的最后，用来把控制权还给`Context`让其它的中间件能执行，反过来说就是如果没有这一句其它的中间件就不能执行，翻阅[这篇文章](https://segmentfault.com/q/1010000020256918)发现不止我有同样的想法，直到今天遇到跨域问题需要为此写一个中间件，于是找到了[cors](https://github.com/gin-contrib/cors)这个项目，在翻阅源码的过程中意外发现代码中竟然没有`c.Next()`环节，顿时有了疑虑，其它中间件和`handler`怎么执行？经过仔细确认发现确实没有后，感觉事情并不简单可能超出认知了于是就有了这篇文章。
>
> 还是一样先实践验证后分析原理，既然[cors](https://github.com/gin-contrib/cors)可以不用`c.Next()`,那我们就来写个小demo验证下如果没有调用`Next()`后续中间件到底能不能执行？

### demo测试

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

func middleware1() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("middleware 1")
	}
}

func middleware2() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("middleware 2")
	}
}

func main() {
	r := gin.Default()
	r.Use(middleware1(), middleware2())
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, "ok")
	})
	r.Run(":8080")
}
```

![](https://img-qingbo.oss-cn-beijing.aliyuncs.com/img/20220718123921.png)

可以发现中间件中最后虽然没有调用`Next()`，但其它中间件及`handlers`也都执行了。

而网上有些人说的所谓使用`c.Next()`使得在`c.Abort()`之后执行其他中间件，更是无稽之谈！

```go
package main

import (
   "log"

   "github.com/gin-gonic/gin"
)

func middleware1() gin.HandlerFunc {
   return func(c *gin.Context) {
      log.Println("middleware 1")
      c.Abort()
      c.Next()
      return
   }
}

func middleware2() gin.HandlerFunc {
   return func(c *gin.Context) {
      log.Println("middleware 2")
   }
}

func main() {
   r := gin.Default()
   r.Use(middleware1(), middleware2())
   r.GET("/ping", func(c *gin.Context) {
      c.JSON(200, "ok")
   })
   r.Run(":8080")
}
```

![](https://img-qingbo.oss-cn-beijing.aliyuncs.com/img/20220718124135.png)

因此可以得出：`c.Next()`跟我最开始理解的一样，就是在中间件执行完后续的一些事情（[Next()方法](https://www.topgoer.com/gin%E6%A1%86%E6%9E%B6/gin%E4%B8%AD%E9%97%B4%E4%BB%B6/next%E6%96%B9%E6%B3%95.html)）。只针对当前的中间件，并不影响其他中间件。

接下来我们来结合源码分析下原理。

### 原理

`Gin`中最终处理请求的逻辑是在`engine.handleHTTPRequest()` 这个函数：

```go
func (engine *Engine) handleHTTPRequest(c *Context) {
	// ...

	// Find root of the tree for the given HTTP method
	t := engine.trees
	for i, tl := 0, len(t); i < tl; i++ {
		if t[i].method != httpMethod {
			continue
		}
		root := t[i].root
		// Find route in tree
		value := root.getValue(rPath, c.params, unescape)
		if value.params != nil {
			c.Params = *value.params
		}
		if value.handlers != nil {
			c.handlers = value.handlers
			c.fullPath = value.fullPath
			c.Next() //执行handlers
			c.writermem.WriteHeaderNow()
			return
		}
		// ...
		}
		break
	}

// ... 
}
```

其中`c.Next()` 是关键：

```go
// Next should be used only inside middleware.
// It executes the pending handlers in the chain inside the calling handler.
// See example in GitHub.
func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c) //执行handler
		c.index++
	}
}
```

从`Next()`方法我们可以看到它会遍历执行全部`handlers`（中间件也是`handler`），所以中间件中调不调用`Next()`方法并不会影响后续中间件的执行。

既然中间件中没有`Next()`不影响后续中间件的执行，那么在当前中间件中调用`c.Next()`的作用又是什么呢？

通过`Next()`函数的逻辑也能很清晰的得出结论：**在当前中间件中调用`c.Next()`时会中断当前中间件中后续的逻辑，转而执行后续的中间件和handlers，等他们全部执行完以后再回来执行当前中间件的后续代码。**

### 结论

`c.Next()`跟我最开始理解的一样，就是在中间件执行完后续的一些事情（[Next()方法](https://www.topgoer.com/gin%E6%A1%86%E6%9E%B6/gin%E4%B8%AD%E9%97%B4%E4%BB%B6/next%E6%96%B9%E6%B3%95.html)）。只针对当前的中间件，并不影响其他中间件。

1. **中间件代码最后即使没有调用`Next()`方法，后续中间件及`handlers`也会执行；**
2. **如果在中间件函数的非结尾调用`Next()`方法当前中间件剩余代码会被暂停执行，会先去执行后续中间件及`handlers`，等这些`handlers`全部执行完以后程序控制权会回到当前中间件继续执行剩余代码；**
3. **如果想提前中止当前中间件的执行应该使用`return`退出而不是`Next()`方法；**
4. **如果想中断剩余中间件及handlers应该使用`Abort`方法，但需要注意当前中间件的剩余代码会继续执行。**

### 参考

https://segmentfault.com/q/1010000020256918

https://www.cnblogs.com/yjf512/p/9670990.html

