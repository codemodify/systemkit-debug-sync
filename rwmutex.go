package syncdebug

import (
	"sync"
)

type RWMutex struct {
	mu sync.RWMutex
}

func (thisRef *RWMutex) Lock() {
	internal.lock(thisRef.mu.Lock, thisRef)
}

func (thisRef *RWMutex) Unlock() {
	thisRef.mu.Unlock()
	if !TraceOptions.Disable {
		postUnlock(thisRef)
	}
}

func (thisRef *RWMutex) RLock() {
	lock(thisRef.mu.RLock, thisRef)
}

func (thisRef *RWMutex) RUnlock() {
	thisRef.mu.RUnlock()
	if !TraceOptions.Disable {
		postUnlock(thisRef)
	}
}

func (thisRef *RWMutex) RLocker() sync.Locker {
	return (*rlocker)(thisRef)
}
