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

// Count increments the counter mapped to key by 1
func (e *EHC) Count(key interface{}) {
	e.CountMultiple(key, 1)
}

// CountMultiple increments the counter mapped to key by the given count
func (e *EHC) CountMultiple(key interface{}, count int64) {
	e.valueLock.RLock()
	counter := e.values[key]
	// does this counter exist?
	if counter != nil {
		// if it does exist, increment it
		counter.inc(count)
		e.valueLock.RUnlock()
		return
	}

	// doesn't exist yet, so let's acquire
	// an exclusive lock to create the counter
	e.valueLock.RUnlock()
	e.valueLock.Lock()

	// we need to check that no one raced us here;
	// the counter may have already been created while
	// we were waiting our turn for the Lock()
	counter = e.values[key]
	if counter == nil {
		// if no one raced us here, let's create the counter
		e.values[key] = newCounter(e, key)
	}
	e.valueLock.Unlock()

	// now we can call Count and have it actually be applied
	e.CountMultiple(key, count)
}

func (e *EHC) remove(key interface{}) {
	e.valueLock.Lock()
	defer e.valueLock.Unlock()

	// let's check to make sure the value wasn't incremented
	// while we were preparing to remove it
	val := e.values[key]
	if val != nil && val.Value() == 0 {
		delete(e.values, key)
	}
}

// Counter is the public interface for what is stored in the map
type Counter interface {
	inc(int64)
	Value() int64
}

// counter is the concrete implementation
type counter struct {
	count  int64
	parent *EHC
	key    interface{}
}

func newCounter(parent *EHC, key interface{}) Counter {
	return &counter{
		parent: parent,
		key:    key,
	}
}

func (c *counter) inc(count int64) {
	if count == 0 {
		return
	}

	atomic.AddInt64(&c.count, count)

	// after the window has elapsed, retract this increment
	time.AfterFunc(c.parent.window, func() {
		value := atomic.AddInt64(&c.count, -count)
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
