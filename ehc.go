package ehc

import (
	"sync"
	"sync/atomic"
	"time"
)

type EHC struct {
	// valueLock controls the values map.
	// a Lock() is required to insert/remove items from the map,
	// but only RLock() is needed to view the map or
	// to edit a counter that's already in the map.
	valueLock sync.RWMutex

	values map[interface{}]Counter

	// window controls the measurement window. Counts expire after this window.
	window time.Duration
}

// NewEHC will return an Expiring Hash Counter. Each increment will be removed
// after the window elapses, allowing you to know that a particular key has been
// counted exactly so many times over the past duration.
func NewEHC(window time.Duration) *EHC {
	return &EHC{
		values: map[interface{}]Counter{},
		window: window,
	}
}

// Values will lock the mutex, then return the map reference and the lock.
// You must unlock it.
func (e *EHC) Values() (map[interface{}]Counter, sync.Locker) {
	e.valueLock.RLock()
	return e.values, e.valueLock.RLocker()
}

func (e *EHC) Count(key interface{}) {
	e.valueLock.RLock()
	counter := e.values[key]
	// does this counter exist?
	if counter == nil {
		// doesn't exist yet
		e.valueLock.RUnlock()
		e.valueLock.Lock()
		defer e.valueLock.Unlock()
		// we need to check that no one raced us here
		counter := e.values[key]
		if counter != nil {
			// if they did, then let's just start over
			e.Count(key)
			return
		}
		// otherwise, let's add the counter
		counter = newCounter(e, key)
		e.values[key] = counter
		return
	}
	// if it does exist, increment it
	counter.inc()
	e.valueLock.RUnlock()
}

func (e *EHC) remove(key interface{}) {
	e.valueLock.Lock()
	delete(e.values, key)
	e.valueLock.Unlock()
}

// Counter is the public interface for what is stored in the map
type Counter interface {
	inc()
	Value() int64
}

// counter is the concrete implementation
type counter struct {
	parent *EHC
	key    interface{}
	count  int64
}

func newCounter(parent *EHC, key interface{}) *counter {
	c := &counter{
		parent: parent,
		key:    key,
	}
	c.inc() //always start a counter at 1
	return c
}

func (c *counter) inc() {
	atomic.AddInt64(&c.count, 1)

	// after the window has elapsed, retract this increment
	time.AfterFunc(c.parent.window, func() {
		value := atomic.AddInt64(&c.count, -1)
		// if we hit zero, remove this counter from the map
		if value == 0 {
			c.parent.remove(c.key)
		}
	})
}

// Value returns the current value held in the atomic counter
func (c *counter) Value() int64 {
	return atomic.LoadInt64(&c.count)
}
