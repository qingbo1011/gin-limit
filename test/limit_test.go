package tets

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/ratelimit"
)

func TestTime(t *testing.T) {
	//var per time.Duration
	duration := time.Duration(100) / time.Duration(100)
	fmt.Println(duration)
}

func TestLeakyBucket(t *testing.T) {
	r1 := ratelimit.New(100) // (10表示1s/10，即每100ms(0.1s)运行一次；同理，1表示1s/1,即每秒运行一次)
	prev := time.Now()
	for i := 0; i < 10; i++ {
		now := r1.Take()
		fmt.Println(i, now.Sub(prev))
		prev = now
	}
}
