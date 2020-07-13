package syncdebug

import (
	"io"
	"os"
	"sync"
	"time"
)

const header = "POTENTIAL DEADLOCK:"

type TraceOpt struct {
	Disable                   bool          // Mutex/RWMutex would work exactly as their sync counterparts, almost no runtime penalty, no deadlock detection if Disable == true
	DisableLockOrderDetection bool          // Would disable lock order based deadlock detection if DisableLockOrderDetection == true
	DeadlockTimeout           time.Duration // Waiting for a lock for longer than DeadlockTimeout is considered a deadlock, Ignored is DeadlockTimeout <= 0
	OnPotentialDeadlock       func()        // called each time a potential deadlock is detected -- either based on lock order or on lock wait time
	MaxMapSize                int           // Will keep MaxMapSize lock pairs (happens before // happens after) in the map, resets once the threshold is reached
	PrintAllCurrentGoroutines bool          // Will dump stacktraces of all goroutines when inconsistent locking is detected
	LogBuf                    io.Writer     // Will print deadlock info to log buffer
	logBufMutex               *sync.Mutex   // Protects the LogBuf
}

var TraceOptions = TraceOpt{
	Disable:                   false,
	DisableLockOrderDetection: false,
	DeadlockTimeout:           time.Second * 30,
	OnPotentialDeadlock:       func() { os.Exit(2) },
	MaxMapSize:                1024 * 64,
	PrintAllCurrentGoroutines: true,
	logBufMutex:               &sync.Mutex{},
	LogBuf:                    os.Stderr,
}
