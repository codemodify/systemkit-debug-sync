package syncdebug

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/petermattis/goid"
)

func preLock(skip int, p interface{}) {
	lo.preLock(skip, p)
}

func postLock(skip int, p interface{}) {
	lo.postLock(skip, p)
}

func postUnlock(p interface{}) {
	lo.postUnlock(p)
}

func lock(lockFn func(), ptr interface{}) {
	if TraceOptions.Disable {
		lockFn()
		return
	}
	preLock(4, ptr)
	if TraceOptions.DeadlockTimeout <= 0 {
		lockFn()
	} else {
		ch := make(chan struct{})
		go func() {
			for {
				t := time.NewTimer(TraceOptions.DeadlockTimeout)
				defer t.Stop() // This runs after the losure finishes, but it's OK.
				select {
				case <-t.C:
					lo.mu.Lock()
					prev, ok := lo.cur[ptr]
					if !ok {
						lo.mu.Unlock()
						break // Nobody seems to be holding the lock, try again.
					}
					TraceOptions.mu.Lock()
					fmt.Fprintln(TraceOptions.LogBuf, header)
					fmt.Fprintln(TraceOptions.LogBuf, "Previous place where the lock was grabbed")
					fmt.Fprintf(TraceOptions.LogBuf, "goroutine %v lock %p\n", prev.gid, ptr)
					printStack(TraceOptions.LogBuf, prev.stack)
					fmt.Fprintln(TraceOptions.LogBuf, "Have been trying to lock it again for more than", TraceOptions.DeadlockTimeout)
					fmt.Fprintf(TraceOptions.LogBuf, "goroutine %v lock %p\n", goid.Get(), ptr)
					printStack(TraceOptions.LogBuf, callers(2))
					stacks := stacks()
					grs := bytes.Split(stacks, []byte("\n\n"))
					for _, g := range grs {
						if goid.ExtractGID(g) == prev.gid {
							fmt.Fprintln(TraceOptions.LogBuf, "Here is what goroutine", prev.gid, "doing now")
							TraceOptions.LogBuf.Write(g)
							fmt.Fprintln(TraceOptions.LogBuf)
						}
					}
					lo.other(ptr)
					if TraceOptions.PrintAllCurrentGoroutines {
						fmt.Fprintln(TraceOptions.LogBuf, "All current goroutines:")
						TraceOptions.LogBuf.Write(stacks)
					}
					fmt.Fprintln(TraceOptions.LogBuf)
					if buf, ok := TraceOptions.LogBuf.(*bufio.Writer); ok {
						buf.Flush()
					}
					TraceOptions.mu.Unlock()
					lo.mu.Unlock()
					TraceOptions.OnPotentialDeadlock()
					<-ch
					return
				case <-ch:
					return
				}
			}
		}()
		lockFn()
		postLock(4, ptr)
		close(ch)
		return
	}
	postLock(4, ptr)
}

type lockOrder struct {
	mu    sync.Mutex
	cur   map[interface{}]stackGID // stacktraces + gids for the locks currently taken.
	order map[beforeAfter]ss       // expected order of locks.
}

type stackGID struct {
	stack []uintptr
	gid   int64
}

type beforeAfter struct {
	before interface{}
	after  interface{}
}

type ss struct {
	before []uintptr
	after  []uintptr
}

var lo = newLockOrder()

func newLockOrder() *lockOrder {
	return &lockOrder{
		cur:   map[interface{}]stackGID{},
		order: map[beforeAfter]ss{},
	}
}

func (l *lockOrder) postLock(skip int, p interface{}) {
	stack := callers(skip)
	gid := goid.Get()
	l.mu.Lock()
	l.cur[p] = stackGID{stack, gid}
	l.mu.Unlock()
}

func (l *lockOrder) preLock(skip int, p interface{}) {
	if TraceOptions.DisableLockOrderDetection {
		return
	}
	stack := callers(skip)
	gid := goid.Get()
	l.mu.Lock()
	for b, bs := range l.cur {
		if b == p {
			if bs.gid == gid {
				TraceOptions.mu.Lock()
				fmt.Fprintln(TraceOptions.LogBuf, header, "Recursive locking:")
				fmt.Fprintf(TraceOptions.LogBuf, "current goroutine %d lock %p\n", gid, b)
				printStack(TraceOptions.LogBuf, stack)
				fmt.Fprintln(TraceOptions.LogBuf, "Previous place where the lock was grabbed (same goroutine)")
				printStack(TraceOptions.LogBuf, bs.stack)
				l.other(p)
				if buf, ok := TraceOptions.LogBuf.(*bufio.Writer); ok {
					buf.Flush()
				}
				TraceOptions.mu.Unlock()
				TraceOptions.OnPotentialDeadlock()
			}
			continue
		}
		if bs.gid != gid { // We want locks taken in the same goroutine only.
			continue
		}
		if s, ok := l.order[beforeAfter{p, b}]; ok {
			TraceOptions.mu.Lock()
			fmt.Fprintln(TraceOptions.LogBuf, header, "Inconsistent locking. saw this ordering in one goroutine:")
			fmt.Fprintln(TraceOptions.LogBuf, "happened before")
			printStack(TraceOptions.LogBuf, s.before)
			fmt.Fprintln(TraceOptions.LogBuf, "happened after")
			printStack(TraceOptions.LogBuf, s.after)
			fmt.Fprintln(TraceOptions.LogBuf, "in another goroutine: happened before")
			printStack(TraceOptions.LogBuf, bs.stack)
			fmt.Fprintln(TraceOptions.LogBuf, "happened after")
			printStack(TraceOptions.LogBuf, stack)
			l.other(p)
			fmt.Fprintln(TraceOptions.LogBuf)
			if buf, ok := TraceOptions.LogBuf.(*bufio.Writer); ok {
				buf.Flush()
			}
			TraceOptions.mu.Unlock()
			TraceOptions.OnPotentialDeadlock()
		}
		l.order[beforeAfter{b, p}] = ss{bs.stack, stack}
		if len(l.order) == TraceOptions.MaxMapSize { // Reset the map to keep memory footprint bounded.
			l.order = map[beforeAfter]ss{}
		}
	}
	l.mu.Unlock()
}

func (l *lockOrder) postUnlock(p interface{}) {
	l.mu.Lock()
	delete(l.cur, p)
	l.mu.Unlock()
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }

// Under lo.mu Locked.
func (l *lockOrder) other(ptr interface{}) {
	empty := true
	for k := range l.cur {
		if k == ptr {
			continue
		}
		empty = false
	}
	if empty {
		return
	}
	fmt.Fprintln(TraceOptions.LogBuf, "Other goroutines holding locks:")
	for k, pp := range l.cur {
		if k == ptr {
			continue
		}
		fmt.Fprintf(TraceOptions.LogBuf, "goroutine %v lock %p\n", pp.gid, k)
		printStack(TraceOptions.LogBuf, pp.stack)
	}
	fmt.Fprintln(TraceOptions.LogBuf)
}
