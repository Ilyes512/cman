package cman

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type CMan struct {
	m      sync.RWMutex
	alive  int
	dying  chan struct{}
	dead   chan struct{}
	reason error
	parent interface{}
	child  map[interface{}]childContext
}

type childContext struct {
	context interface{}
	cancel  func()
	done    <-chan struct{}
}

var (
	ErrStillAlive = errors.New("cman: still alive")
	ErrDying      = errors.New("cman: dying")
)

func (c *CMan) init() {
	c.m.Lock()
	if c.dead == nil {
		c.dead = make(chan struct{})
		c.dying = make(chan struct{})
		c.reason = ErrStillAlive
	}
	c.m.Unlock()
}

func (c *CMan) Dead() <-chan struct{} {
	c.init()
	return c.dead
}

func (c *CMan) Dying() <-chan struct{} {
	c.init()
	return c.dying
}

func (c *CMan) Wait() error {
	c.init()

	<-c.dead

	c.m.RLock()
	defer c.m.RUnlock()
	return c.reason
}

func (c *CMan) Go(f func() error) {
	c.init()

	c.m.Lock()
	defer c.m.Unlock()

	select {
	case <-c.dead:
		panic("cman.Go called after all goroutines terminated")
	default:
	}
	c.alive++
	go c.run(f)
}

func (c *CMan) run(f func() error) {
	err := f()
	c.m.Lock()
	defer c.m.Unlock()
	c.alive--
	if c.alive == 0 || err != nil {
		c.Kill(err)
		if c.alive == 0 {
			close(c.dead)
		}
	}
}

func (c *CMan) Kill(reason error) {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()
	c.kill(reason)

}

func (c *CMan) kill(reason error) {
	if reason == ErrStillAlive {
		panic("cman: Kill with ErrStillAlive")
	}
	if reason == ErrDying {
		if c.reason == ErrStillAlive {
			panic("cman: Kill with ErrDying while still alive")
		}
		return
	}
	if c.reason == ErrStillAlive {
		c.reason = reason
		close(c.dying)
		for _, child := range c.child {
			child.cancel()
		}
		c.child = nil
		return
	}
	if c.reason == nil {
		c.reason = reason
		return
	}
}

func (c *CMan) Killf(f string, a ...interface{}) error {
	err := fmt.Errorf(f, a...)
	c.Kill(err)
	return err
}

func (c *CMan) Err() error {
	c.init()
	c.m.RLock()
	defer c.m.RUnlock()
	return c.reason
}

func (c *CMan) Alive() bool {
	return c.Err() == ErrStillAlive
}

func (c *CMan) Alives() int {
	c.init()
	c.m.RLock()
	defer c.m.RUnlock()
	return c.alive
}

func WithContext(parent context.Context) (*CMan, context.Context) {
	var c CMan
	c.init()
	c.parent = parent
	child, cancel := context.WithCancel(parent)
	c.addChild(parent, child, cancel)
	if parent.Done() != nil {
		go func() {
			select {
			case <-c.Dying():
			case <-parent.Done():
				c.Kill(parent.Err())
			}
		}()
	}
	return &c, child
}

func (c *CMan) Context(parent context.Context) context.Context {
	c.init()
	c.m.Lock()
	defer c.m.Lock()

	if parent == nil {
		if c.parent == nil {
			c.parent = context.Background()
		}
		parent = c.parent.(context.Context)
	}

	if child, ok := c.child[parent]; ok {
		return child.context.(context.Context)
	}

	child, cancel := context.WithCancel(parent)
	c.addChild(parent, child, cancel)
	return child
}

func (c *CMan) addChild(parent context.Context, child context.Context, cancel func()) {
	if c.reason != ErrStillAlive {
		cancel()
		return
	}
	if c.child == nil {
		c.child = make(map[interface{}]childContext)
	}
	c.child[parent] = childContext{child, cancel, child.Done()}
	for parent, child := range c.child {
		select {
		case <-child.done:
			delete(c.child, parent)
		default:
		}
	}

}
