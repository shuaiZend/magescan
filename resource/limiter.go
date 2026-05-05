package resource

import (
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// Limiter monitors and controls resource usage
type Limiter struct {
	cpuLimit      int   // Max CPU cores (0 = no limit)
	memLimitMB    int64 // Max memory in MB (0 = no limit)
	throttleCh    chan struct{}
	stopCh        chan struct{}
	isThrottled   atomic.Bool
	once          sync.Once
	originalProcs int
}

// NewLimiter creates a resource limiter with the given constraints
// cpuLimit: max CPU cores to use (0 = use all available)
// memLimitMB: max memory in MB (0 = no limit)
func NewLimiter(cpuLimit int, memLimitMB int) *Limiter {
	return &Limiter{
		cpuLimit:   cpuLimit,
		memLimitMB: int64(memLimitMB),
		throttleCh: make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
	}
}

// Start begins resource monitoring in a background goroutine
func (l *Limiter) Start() {
	// Apply CPU limit
	if l.cpuLimit > 0 && l.cpuLimit <= runtime.NumCPU() {
		l.originalProcs = runtime.GOMAXPROCS(l.cpuLimit)
	} else {
		l.originalProcs = runtime.GOMAXPROCS(0) // read current without changing
	}

	go l.monitor()
}

// Stop halts resource monitoring and restores original settings
func (l *Limiter) Stop() {
	l.once.Do(func() {
		close(l.stopCh)
		runtime.GOMAXPROCS(l.originalProcs)
	})
}

// ThrottleChannel returns the channel that workers should check for throttling
func (l *Limiter) ThrottleChannel() chan struct{} {
	return l.throttleCh
}

// IsThrottled returns whether workers are currently being throttled
func (l *Limiter) IsThrottled() bool {
	return l.isThrottled.Load()
}

func (l *Limiter) monitor() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.checkMemory()
		}
	}
}

func (l *Limiter) checkMemory() {
	if l.memLimitMB <= 0 {
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := int64(m.Alloc / (1024 * 1024))

	if allocMB > l.memLimitMB {
		// Over limit: throttle workers
		l.isThrottled.Store(true)

		// Signal throttle (non-blocking)
		select {
		case l.throttleCh <- struct{}{}:
		default:
		}

		// Force GC to reclaim memory
		runtime.GC()
		debug.FreeOSMemory()

		// Give GC time to work
		time.Sleep(1 * time.Second)
	} else if l.isThrottled.Load() {
		// Hysteresis: only un-throttle when memory drops to 80% of limit
		threshold := l.memLimitMB * 80 / 100
		if allocMB <= threshold {
			l.isThrottled.Store(false)

			// Drain throttleCh to unblock workers
			select {
			case <-l.throttleCh:
			default:
			}
		}
	}
}
