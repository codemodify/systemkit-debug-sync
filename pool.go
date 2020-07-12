package syncdebug

import "sync"

type Pool struct {
	sync.Pool
}
