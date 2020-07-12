package syncdebug

import (
	"sync"
)

type Mutex struct {
	mu sync.Mutex
}

func (thisRef *Mutex) Lock() {
	lock(thisRef.mu.Lock, thisRef)
}

func (thisRef *Mutex) Unlock() {
	thisRef.mu.Unlock()
	if !Opts.Disable {
		postUnlock(thisRef)
	}
}
