package mbserver

import (
	"net"
	"sync"
	"time"
)

// Watchdog 구조체 정의
type Watchdog struct {
	mu          sync.Mutex
	lastRequest map[net.Conn]time.Time
	timeout     time.Duration
	callback    func(net.Conn)
}

// NewWatchdog 함수 정의
func NewWatchdog(timeout time.Duration, callback func(net.Conn)) *Watchdog {
	return &Watchdog{
		lastRequest: make(map[net.Conn]time.Time),
		timeout:     timeout,
		callback:    callback,
	}
}

// Feed 메서드 정의 (Watchdog 리셋)
func (wd *Watchdog) Feed(conn net.Conn) {
	wd.mu.Lock()
	defer wd.mu.Unlock()
	wd.lastRequest[conn] = time.Now()
}

// Remove 메서드 정의 (Watchdog 제거)
func (wd *Watchdog) Remove(conn net.Conn) {
	wd.mu.Lock()
	defer wd.mu.Unlock()
	delete(wd.lastRequest, conn)
}

// Start 메서드 정의 (Watchdog 시작)
func (wd *Watchdog) Start() {
	go func() {
		for {
			time.Sleep(wd.timeout / 2)
			wd.mu.Lock()
			for conn, lastRequest := range wd.lastRequest {
				if time.Since(lastRequest) > wd.timeout {
					wd.callback(conn)
				}
			}
			wd.mu.Unlock()
		}
	}()
}
