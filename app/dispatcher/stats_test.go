package dispatcher_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	. "v2ray.com/core/app/dispatcher"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
)

type TestCounter struct {
	value int64
	ips   sync.Map
}

func (c *TestCounter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

func (c *TestCounter) Add(v int64) int64 {
	return atomic.AddInt64(&c.value, v)
}

func (c *TestCounter) Set(v int64) int64 {
	return atomic.SwapInt64(&c.value, v)
}
func (c *TestCounter) AddIP(v string) {
	c.ips.Store(v, 1)
	c.ips.Store("last_time", time.Now().Unix())
}
func (c *TestCounter) RemoveAllIPs() string {
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
func (c *TestCounter) GetALLIPs() string {
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
func (c *TestCounter) GetLastIPTime() (int64, bool) {
	var time interface{}
	var ok bool

	time, ok = c.ips.Load("last_time")
	if time != nil {
		return time.(int64), ok
	} else {
		return -1, ok
	}
}
func TestStatsWriter(t *testing.T) {
	var c TestCounter
	writer := &SizeStatWriter{
		Counter: &c,
		Writer:  buf.Discard,
	}

	mb := buf.MergeBytes(nil, []byte("abcd"))
	common.Must(writer.WriteMultiBuffer(mb))

	mb = buf.MergeBytes(nil, []byte("efg"))
	common.Must(writer.WriteMultiBuffer(mb))

	if c.Value() != 7 {
		t.Fatal("unexpected counter value. want 7, but got ", c.Value())
	}
	c.AddIP("1237.0.0.1")
	c.AddIP("123.0.0.1")
	println(c.GetALLIPs())
}
