# ![](https://fonts.gstatic.com/s/i/materialiconsoutlined/flare/v4/24px.svg) Debug Sync Objects
[![](https://img.shields.io/github/v/release/codemodify/systemkit-debug-sync?style=flat-square)](https://github.com/codemodify/systemkit-debug-sync/releases/latest)
![](https://img.shields.io/github/languages/code-size/codemodify/systemkit-debug-sync?style=flat-square)
![](https://img.shields.io/github/last-commit/codemodify/systemkit-debug-sync?style=flat-square)
[![](https://img.shields.io/badge/license-0--license-brightgreen?style=flat-square)](https://github.com/codemodify/TheFreeLicense)

![](https://img.shields.io/github/workflow/status/codemodify/systemkit-debug-sync/qa?style=flat-square)
![](https://img.shields.io/github/issues/codemodify/systemkit-debug-sync?style=flat-square)
[![](https://goreportcard.com/badge/github.com/codemodify/systemkit-debug-sync?style=flat-square)](https://goreportcard.com/report/github.com/codemodify/systemkit-debug-sync)

[![](https://img.shields.io/badge/godoc-reference-brightgreen?style=flat-square)](https://godoc.org/github.com/codemodify/systemkit-debug-sync)
![](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)
![](https://img.shields.io/gitter/room/codemodify/systemkit-debug-sync?style=flat-square)

![](https://img.shields.io/github/contributors/codemodify/systemkit-debug-sync?style=flat-square)
![](https://img.shields.io/github/stars/codemodify/systemkit-debug-sync?style=flat-square)
![](https://img.shields.io/github/watchers/codemodify/systemkit-debug-sync?style=flat-square)
![](https://img.shields.io/github/forks/codemodify/systemkit-debug-sync?style=flat-square)


# ![](https://fonts.gstatic.com/s/i/materialicons/extension/v5/24px.svg) What
Drop-in replacement for `sync.*` types with deadlock detection

### Normal Use
```go
import "github.com/codemodify/systemkit-debug-sync"
var mutex syncdebug.Mutex

mutex.Lock()
defer mutex.Unlock()
```

### Dead-Lock Scenario 1
```go
go func(){
	...
	mutex1.Lock()
	mutex2.Lock()
	...
}()

go func(){
	...
	mutex2.Lock() // lock order reversed, DEAD-LOCK
	mutex1.Lock()
	...
}()
```

### Dead-Lock Scenario 2
```go
go func(){
	...
	mutex1.Lock()
	mutex2.Lock()
	...

	...
	mutex1.Lock() // lock order reversed, DEAD-LOCK
	mutex2.Lock()
	...
}()
```

