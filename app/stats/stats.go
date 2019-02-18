// +build !confonly

package stats

//go:generate errorgen

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"v2ray.com/core/features/stats"
)

// Counter is an implementation of stats.Counter.
type Counter struct {
	value int64
	ips   sync.Map
}

// Value implements stats.Counter.
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Set implements stats.Counter.
func (c *Counter) Set(newValue int64) int64 {
	return atomic.SwapInt64(&c.value, newValue)
}

// Add implements stats.Counter.
func (c *Counter) Add(delta int64) int64 {
	return atomic.AddInt64(&c.value, delta)
}

// Add ips
func (c *Counter) AddIP(ip string) {
	c.ips.Store(ip, 1)
	c.ips.Store("last_time", time.Now().Unix())
}

// Remove and return all ips
func (c *Counter) RemoveAllIPs() string {
	var allips strings.Builder
	c.ips.Range(func(key interface{}, value interface{}) bool {
		if key.(string) != "last_time" {
			allips.WriteString(";")
			allips.WriteString(key.(string))
			c.ips.Delete(key)
		}
		return true
	})
	return allips.String()
}

func (c *Counter) GetALLIPs() string {
	var allips strings.Builder
	c.ips.Range(func(key interface{}, value interface{}) bool {
		if key.(string) != "last_time" {
			allips.WriteString(";")
			allips.WriteString(key.(string))
		}
		return true
	})
	return allips.String()
}
func (c *Counter) GetLastIPTime() (int64, bool) {
	var time interface{}
	var ok bool

	time, ok = c.ips.Load("last_time")
	if time != nil {
		return time.(int64), ok
	} else {
		return -1, ok
	}
}

// Manager is an implementation of stats.Manager.
type Manager struct {
	access   sync.RWMutex
	counters map[string]*Counter
}

func NewManager(ctx context.Context, config *Config) (*Manager, error) {
	m := &Manager{
		counters: make(map[string]*Counter),
	}

	return m, nil
}

func (*Manager) Type() interface{} {
	return stats.ManagerType()
}

func (m *Manager) RegisterCounter(name string) (stats.Counter, error) {
	m.access.Lock()
	defer m.access.Unlock()

	if _, found := m.counters[name]; found {
		return nil, newError("Counter ", name, " already registered.")
	}
	newError("create new counter ", name).AtDebug().WriteToLog()
	c := new(Counter)
	m.counters[name] = c
	return c, nil
}

func (m *Manager) GetCounter(name string) stats.Counter {
	m.access.RLock()
	defer m.access.RUnlock()

	if c, found := m.counters[name]; found {
		return c
	}
	return nil
}

func (m *Manager) Visit(visitor func(string, stats.Counter) bool) {
	m.access.RLock()
	defer m.access.RUnlock()

	for name, c := range m.counters {
		if !visitor(name, c) {
			break
		}
	}
}

// Start implements common.Runnable.
func (m *Manager) Start() error {
	return nil
}

// Close implement common.Closable.
func (m *Manager) Close() error {
	return nil
}
