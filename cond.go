package syncdebug

import "sync"

type Cond struct {
	sync.Cond
}
