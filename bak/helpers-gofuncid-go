package syncdebug

import (
	"bytes"
	"runtime"
	"strconv"
)

func goFuncID() int64

func goFuncIDFromRawStack(s []byte) int64 {

	// Parse the "go func()" ID from runtime.Stack() output
	// Slow, but it works

	s = s[len("goroutine "):]
	s = s[:bytes.IndexByte(s, ' ')]
	gid, _ := strconv.ParseInt(string(s), 10, 64)
	return gid
}

func goFuncIDSlow() int64 {
	var buf [64]byte
	return goFuncIDFromRawStack(buf[:runtime.Stack(buf[:], false)])
}
