package core

import (
	"math/rand"
	"time"
)

// 站源请求的重试节奏。原实现是 attempt 秒的线性退避（1s、2s、3s…），
// 既不能快速放弃明显不通的域名，又会在网络抖动时让一批任务同时重试。
// 这里改为指数退避并加入抖动：0.8s、1.6s、3.2s…，上限 8s，抖动 ±25%。
const (
	providerRetryBaseDelay = 800 * time.Millisecond
	providerRetryMaxDelay  = 8 * time.Second
)

// providerRetryDelay 返回第 attempt 次重试（attempt 从 1 开始）前的等待时长。
func providerRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	delay := providerRetryBaseDelay
	for index := 1; index < attempt; index++ {
		delay *= 2
		if delay >= providerRetryMaxDelay {
			delay = providerRetryMaxDelay
			break
		}
	}
	jitter := time.Duration(rand.Int63n(int64(delay)/2+1)) - delay/4
	if next := delay + jitter; next > 0 {
		return next
	}
	return delay
}

// providerQuickRetryDelay 用于"服务端瞬时丢弃请求"这类快速失败场景：
// 红果 App 接口的空响应通常在 200ms 内返回，此时用秒级退避会让 8 次重试
// 累计等待超过 30 秒。这里改为 150~400ms 的短抖动。
func providerQuickRetryDelay() time.Duration {
	return 150*time.Millisecond + time.Duration(rand.Int63n(int64(250*time.Millisecond)))
}
