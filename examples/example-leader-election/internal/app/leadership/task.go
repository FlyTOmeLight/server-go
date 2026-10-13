package leadership

import (
	"context"
	"fmt"
	"time"
)

// 模拟一个任务执行函数
func Task(ctx context.Context) {
	// 执行任务，任务重通常包含循环，可以在循环中判断context是否结束，来终止任务
	// 不包含循环的，需要在安全的时候尽快退出任务
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("task is cancelled")
			return
		case <-time.After(time.Second):
			fmt.Println(i)
		}
	}
}
