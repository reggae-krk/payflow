package main

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reggae-krk/payflow/internal/config"
	"golang.org/x/time/rate"
)

type visitors struct {
	mu       sync.Mutex
	visitors map[string]*entry
	config   config.RateLimiterConfig
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func (myVisitors *visitors) getVisitor(ip string) *rate.Limiter {
	myVisitors.mu.Lock()
	defer myVisitors.mu.Unlock()

	v, exists := myVisitors.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Every(myVisitors.config.Interval), int(myVisitors.config.Burst))
		myVisitors.visitors[ip] = &entry{limiter, time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func (myVisitors *visitors) janitor(ctx context.Context) {
	ticker := time.NewTicker(myVisitors.config.CleanupTime)
	defer ticker.Stop()
	tickerChann := ticker.C
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-tickerChann:
			if ctx.Err() != nil {
				break loop
			}
			myVisitors.mu.Lock()
			for ip, v := range myVisitors.visitors {
				if time.Since(v.lastSeen) > myVisitors.config.IdleTTL {
					delete(myVisitors.visitors, ip)
				}
			}
			myVisitors.mu.Unlock()
		}
	}
}

func RateLimiter(ctx context.Context, rateLimiterCfg config.RateLimiterConfig) gin.HandlerFunc {
	myVisitors := visitors{visitors: make(map[string]*entry),
		config: rateLimiterCfg}
	go myVisitors.janitor(ctx)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		limiter := myVisitors.getVisitor(ip)
		reserv := limiter.Reserve()
		if reserv.Delay() > 0 {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(reserv.Delay().Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			reserv.Cancel()
			return
		}
		c.Next()
	}
}
