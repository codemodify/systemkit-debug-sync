package syncdebug

import "sync"

type WaitGroup struct {
	sync.WaitGroup
}
