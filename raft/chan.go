package main

import "sync"

type Channel[V any] struct {
	curChan chan V
	open    bool
	lock    *sync.Mutex
}

func NewChannel[V any]() *Channel[V] {
	return &Channel[V]{
		curChan: make(chan V, 10),
		open:    true,
		lock:    new(sync.Mutex),
	}
}

func (c *Channel[V]) Open() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if !c.open {
		c.curChan = make(chan V, 10)
		c.open = true
	}
}

func (c *Channel[V]) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.open {
		close(c.curChan)
		c.open = false
	}
}

func (c *Channel[V]) Ch() chan V {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.curChan
}

func (c *Channel[V]) Add(element V) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.open {
		c.curChan <- element
	}
}

func (c *Channel[V]) Reset() {
	c.lock.Lock()
	defer c.lock.Unlock()

	close(c.curChan)
	c.curChan = make(chan V, 10)
	c.open = true
}
