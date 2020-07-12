package syncdebug

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const header = "POTENTIAL DEADLOCK:"

var Opts = struct {
	Disable                   bool          // Mutex/RWMutex would work exactly as their sync counterparts, almost no runtime penalty, no deadlock detection if Disable == true
	DisableLockOrderDetection bool          // Would disable lock order based deadlock detection if DisableLockOrderDetection == true
	DeadlockTimeout           time.Duration // Waiting for a lock for longer than DeadlockTimeout is considered a deadlock, Ignored is DeadlockTimeout <= 0
	OnPotentialDeadlock       func()        // called each time a potential deadlock is detected -- either based on lock order or on lock wait time
	MaxMapSize                int           // Will keep MaxMapSize lock pairs (happens before // happens after) in the map, resets once the threshold is reached
	PrintAllCurrentGoroutines bool          // Will dump stacktraces of all goroutines when inconsistent locking is detected
	mu                        *sync.Mutex   // Protects the LogBuf
	LogBuf                    io.Writer     // Will print deadlock info to log buffer
}{
	DeadlockTimeout: time.Second * 30,
	OnPotentialDeadlock: func() {
		os.Exit(2)
	},
	MaxMapSize: 1024 * 64,
	mu:         &sync.Mutex{},
	LogBuf:     os.Stderr,
}

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
	if Opts.Disable {
		lockFn()
		return
	}
	preLock(4, ptr)
	if Opts.DeadlockTimeout <= 0 {
		lockFn()
	} else {
		ch := make(chan struct{})
		go func() {
			for {
				t := time.NewTimer(Opts.DeadlockTimeout)
				defer t.Stop() // This runs after the losure finishes, but it's OK.
				select {
				case <-t.C:
					lo.mu.Lock()
					prev, ok := lo.cur[ptr]
					if !ok {
						lo.mu.Unlock()
						break // Nobody seems to be holding the lock, try again.
					}
					Opts.mu.Lock()
					fmt.Fprintln(Opts.LogBuf, header)
					fmt.Fprintln(Opts.LogBuf, "Previous place where the lock was grabbed")
					fmt.Fprintf(Opts.LogBuf, "goroutine %v lock %p\n", prev.gid, ptr)
					printStack(Opts.LogBuf, prev.stack)
					fmt.Fprintln(Opts.LogBuf, "Have been trying to lock it again for more than", Opts.DeadlockTimeout)
					fmt.Fprintf(Opts.LogBuf, "goroutine %v lock %p\n", goid.Get(), ptr)
					printStack(Opts.LogBuf, callers(2))
					stacks := stacks()
					grs := bytes.Split(stacks, []byte("\n\n"))
					for _, g := range grs {
						if goid.ExtractGID(g) == prev.gid {
							fmt.Fprintln(Opts.LogBuf, "Here is what goroutine", prev.gid, "doing now")
							Opts.LogBuf.Write(g)
							fmt.Fprintln(Opts.LogBuf)
						}
					}
					lo.other(ptr)
					if Opts.PrintAllCurrentGoroutines {
						fmt.Fprintln(Opts.LogBuf, "All current goroutines:")
						Opts.LogBuf.Write(stacks)
					}
					fmt.Fprintln(Opts.LogBuf)
					if buf, ok := Opts.LogBuf.(*bufio.Writer); ok {
						buf.Flush()
					}
					Opts.mu.Unlock()
					lo.mu.Unlock()
					Opts.OnPotentialDeadlock()
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
	if Opts.DisableLockOrderDetection {
		return
	}
	stack := callers(skip)
	gid := goid.Get()
	l.mu.Lock()
	for b, bs := range l.cur {
		if b == p {
			if bs.gid == gid {
				Opts.mu.Lock()
				fmt.Fprintln(Opts.LogBuf, header, "Recursive locking:")
				fmt.Fprintf(Opts.LogBuf, "current goroutine %d lock %p\n", gid, b)
				printStack(Opts.LogBuf, stack)
				fmt.Fprintln(Opts.LogBuf, "Previous place where the lock was grabbed (same goroutine)")
				printStack(Opts.LogBuf, bs.stack)
				l.other(p)
				if buf, ok := Opts.LogBuf.(*bufio.Writer); ok {
					buf.Flush()
				}
				Opts.mu.Unlock()
				Opts.OnPotentialDeadlock()
			}
			continue
		}
		if bs.gid != gid { // We want locks taken in the same goroutine only.
			continue
		}
		if s, ok := l.order[beforeAfter{p, b}]; ok {
			Opts.mu.Lock()
			fmt.Fprintln(Opts.LogBuf, header, "Inconsistent locking. saw this ordering in one goroutine:")
			fmt.Fprintln(Opts.LogBuf, "happened before")
			printStack(Opts.LogBuf, s.before)
			fmt.Fprintln(Opts.LogBuf, "happened after")
			printStack(Opts.LogBuf, s.after)
			fmt.Fprintln(Opts.LogBuf, "in another goroutine: happened before")
			printStack(Opts.LogBuf, bs.stack)
			fmt.Fprintln(Opts.LogBuf, "happened after")
			printStack(Opts.LogBuf, stack)
			l.other(p)
			fmt.Fprintln(Opts.LogBuf)
			if buf, ok := Opts.LogBuf.(*bufio.Writer); ok {
				buf.Flush()
			}
			Opts.mu.Unlock()
			Opts.OnPotentialDeadlock()
		}
		l.order[beforeAfter{b, p}] = ss{bs.stack, stack}
		if len(l.order) == Opts.MaxMapSize { // Reset the map to keep memory footprint bounded.
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
	fmt.Fprintln(Opts.LogBuf, "Other goroutines holding locks:")
	for k, pp := range l.cur {
		if k == ptr {
			continue
		}
		fmt.Fprintf(Opts.LogBuf, "goroutine %v lock %p\n", pp.gid, k)
		printStack(Opts.LogBuf, pp.stack)
	}
	fmt.Fprintln(Opts.LogBuf)
}
