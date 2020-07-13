package syncdebug

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	callstack "github.com/codemodify/systemkit-callstack"
)

func preLock(skip int, p interface{}) {
	lockOrderTracing.preLock(skip, p)
}

func postLock(skip int, p interface{}) {
	lockOrderTracing.postLock(skip, p)
}

func postUnlock(p interface{}) {
	lockOrderTracing.postUnlock(p)
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
					lockOrderTracing.mutex.Lock()
					prev, ok := lockOrderTracing.current[ptr]
					if !ok {
						lockOrderTracing.mutex.Unlock()
						break // Nobody seems to be holding the lock, try again.
					}
					TraceOptions.logBufMutex.Lock()
					{
						fmt.Fprintln(TraceOptions.LogBuf, header)
						fmt.Fprintln(TraceOptions.LogBuf, "Previous place where the lock was grabbed")
						fmt.Fprintf(TraceOptions.LogBuf, "goroutine %v lock %p\n", prev.gid, ptr)
						fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(prev.stack)))
						fmt.Fprintln(TraceOptions.LogBuf, "Have been trying to lock it again for more than", TraceOptions.DeadlockTimeout)
						fmt.Fprintf(TraceOptions.LogBuf, "goroutine %v lock %p\n", goFuncID(), ptr)
						fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(callstack.GetRawFrames(2+2))))
						stacks := allStacks()
						grs := bytes.Split(stacks, []byte("\n\n"))
						for _, g := range grs {
							if goFuncIDFromRawStack(g) == prev.gid {
								fmt.Fprintln(TraceOptions.LogBuf, "Here is what goroutine", prev.gid, "doing now")
								TraceOptions.LogBuf.Write(g)
								fmt.Fprintln(TraceOptions.LogBuf)
							}
						}
						lockOrderTracing.other(ptr)
						if TraceOptions.PrintAllCurrentGoroutines {
							fmt.Fprintln(TraceOptions.LogBuf, "All current goroutines:")
							TraceOptions.LogBuf.Write(stacks)
						}
						fmt.Fprintln(TraceOptions.LogBuf)
						if buf, ok := TraceOptions.LogBuf.(*bufio.Writer); ok {
							buf.Flush()
						}
					}
					TraceOptions.logBufMutex.Unlock()
					lockOrderTracing.mutex.Unlock()
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

// ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~
// lockOrder
// ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~
type stackGofuncidKeyValue struct {
	stack []uintptr
	gid   int64
}

type beforeAfterSomething struct {
	before interface{}
	after  interface{}
}

type beforeAfterStaks struct {
	before []uintptr
	after  []uintptr
}

type lockOrder struct {
	current map[interface{}]stackGofuncidKeyValue     // stacktraces + gids for the locks currently taken
	order   map[beforeAfterSomething]beforeAfterStaks // expected order of locks
	mutex   sync.Mutex                                //
}

var lockOrderTracing = &lockOrder{
	current: map[interface{}]stackGofuncidKeyValue{},
	order:   map[beforeAfterSomething]beforeAfterStaks{},
	mutex:   sync.Mutex{},
}

func (l *lockOrder) postLock(skip int, p interface{}) {
	stack := callstack.GetRawFrames(skip + 2)
	gid := goFuncID()
	l.mutex.Lock()
	l.current[p] = stackGofuncidKeyValue{stack, gid}
	l.mutex.Unlock()
}

func (l *lockOrder) preLock(skip int, p interface{}) {
	if TraceOptions.DisableLockOrderDetection {
		return
	}
	stack := callstack.GetRawFrames(skip + 2)
	gid := goFuncID()
	l.mutex.Lock()
	for b, bs := range l.current {
		if b == p {
			if bs.gid == gid {
				TraceOptions.logBufMutex.Lock()
				{
					fmt.Fprintln(TraceOptions.LogBuf, header, "Recursive locking:")
					fmt.Fprintf(TraceOptions.LogBuf, "current goroutine %d lock %p\n", gid, b)
					fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(stack)))
					fmt.Fprintln(TraceOptions.LogBuf, "Previous place where the lock was grabbed (same goroutine)")
					fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(bs.stack)))
					l.other(p)
					if buf, ok := TraceOptions.LogBuf.(*bufio.Writer); ok {
						buf.Flush()
					}
				}
				TraceOptions.logBufMutex.Unlock()
				TraceOptions.OnPotentialDeadlock()
			}
			continue
		}
		if bs.gid != gid { // We want locks taken in the same goroutine only.
			continue
		}
		if s, ok := l.order[beforeAfterSomething{p, b}]; ok {
			TraceOptions.logBufMutex.Lock()
			{
				fmt.Fprintln(TraceOptions.LogBuf, header, "Inconsistent locking. saw this ordering in one goroutine:")
				fmt.Fprintln(TraceOptions.LogBuf, "happened before")
				fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(s.before)))

				fmt.Fprintln(TraceOptions.LogBuf, "happened after")
				fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(s.after)))

				fmt.Fprintln(TraceOptions.LogBuf, "in another goroutine: happened before")
				fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(bs.stack)))

				fmt.Fprintln(TraceOptions.LogBuf, "happened after")
				fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(stack)))

				l.other(p)
				fmt.Fprintln(TraceOptions.LogBuf)
				if buf, ok := TraceOptions.LogBuf.(*bufio.Writer); ok {
					buf.Flush()
				}
			}
			TraceOptions.logBufMutex.Unlock()
			TraceOptions.OnPotentialDeadlock()
		}
		l.order[beforeAfterSomething{b, p}] = beforeAfterStaks{bs.stack, stack}
		if len(l.order) == TraceOptions.MaxMapSize { // Reset the map to keep memory footprint bounded.
			l.order = map[beforeAfterSomething]beforeAfterStaks{}
		}
	}
	l.mutex.Unlock()
}

func (l *lockOrder) postUnlock(p interface{}) {
	l.mutex.Lock()
	delete(l.current, p)
	l.mutex.Unlock()
}

func (l *lockOrder) other(ptr interface{}) {
	empty := true
	for k := range l.current {
		if k == ptr {
			continue
		}
		empty = false
	}
	if empty {
		return
	}
	fmt.Fprintln(TraceOptions.LogBuf, "Other goroutines holding locks:")
	for k, pp := range l.current {
		if k == ptr {
			continue
		}
		fmt.Fprintf(TraceOptions.LogBuf, "goroutine %v lock %p\n", pp.gid, k)
		fmt.Fprintln(TraceOptions.LogBuf, framesToString(callstack.GetFramesFromRawFrames(pp.stack)))
	}
	fmt.Fprintln(TraceOptions.LogBuf)
}

// ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~
// stack
// ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~ ~~~~
func allStacks() []byte {
	buf := make([]byte, 1024*16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

func framesToString(frames []callstack.Frame) string {
	data, _ := json.MarshalIndent(frames, "   ", "\t")
	return string(data)
}
