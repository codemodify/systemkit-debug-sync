package syncdebug

import (
	"sync"
)

type RWMutex struct {
	mu sync.RWMutex
}

func (thisRef *RWMutex) Lock() {
	lock(thisRef.mu.Lock, thisRef)
}

func (thisRef *RWMutex) Unlock() {
	thisRef.mu.Unlock()
	if !Opts.Disable {
		postUnlock(thisRef)
	}
}

func (thisRef *RWMutex) RLock() {
	lock(thisRef.mu.RLock, thisRef)
}

func (thisRef *RWMutex) RUnlock() {
	thisRef.mu.RUnlock()
	if !Opts.Disable {
		postUnlock(thisRef)
	}
}

func (thisRef *RWMutex) RLocker() sync.Locker {
	return (*rlocker)(thisRef)
}
