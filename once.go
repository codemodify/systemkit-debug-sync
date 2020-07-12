package syncdebug

import "sync"

type Once struct {
	sync.Once
}
