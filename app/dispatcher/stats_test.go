package dispatcher_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	. "v2ray.com/core/app/dispatcher"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/ext/assert"
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
}
func (c *TestCounter) RemoveAll() string {
	var allips strings.Builder
	c.ips.Range(func(key interface{}, value interface{}) bool {
		allips.WriteString(";")
		allips.WriteString(key.(string))
		c.ips.Delete(key)
		return true
	})
	return allips.String()
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
	c.AddIP("123.0.0.1")
	c.AddIP("124.0.0.1")
	assert_local := assert.With(t)
	assert_local(c.RemoveAll(), assert.Equals, ";123.0.0.1;124.0.0.1")

}
