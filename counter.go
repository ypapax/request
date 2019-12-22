package request

import (
	"fmt"
	"sync"
	"time"
)

type Counter struct {
	sync.Mutex
	Success    int
	Fail       int
	Started    time.Time
	LastOk     time.Time
	LastFailed time.Time
}

func (c *Counter) Ok() {
	c.Lock()
	defer c.Unlock()
	c.Success++
	c.LastOk = time.Now()
}
func (c *Counter) Failed() {
	c.Lock()
	defer c.Unlock()
	c.Fail++
	c.LastFailed = time.Now()
}

func (c Counter) String() string {
	c.Lock()
	defer c.Unlock()
	return fmt.Sprintf("Success (%s): %d, Failed (%s): %d, since %s", time.Since(c.LastOk), c.Success, time.Since(c.LastFailed), c.Fail, time.Since(c.Started))
}
