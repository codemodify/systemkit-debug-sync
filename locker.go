package syncdebug

import "sync"

type Locker struct {
	sync.Locker
}
