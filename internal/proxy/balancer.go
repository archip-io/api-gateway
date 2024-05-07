package proxy

import (
	"errors"
	"sync"
	"sync/atomic"
)

type Balancer struct {
	backends     []*Backend
	backsIndexes map[*Backend]int
	cur          atomic.Int32
	rw           sync.RWMutex
}

func NewBalancer() *Balancer {
	b := &Balancer{
		backends:     make([]*Backend, 0),
		cur:          atomic.Int32{},
		backsIndexes: make(map[*Backend]int),
	}
	b.cur.Store(0)
	return b
}

const maxIter = 1000

func (b *Balancer) GetBack() (*Backend, error) {

	for iter := 0; iter < maxIter; iter++ {
		i := int(b.cur.Add(1) - 1)

		b.rw.RLock()

		if len(b.backends) == 0 {
			b.rw.RUnlock()
			return nil, errors.New("no backends available")
		}
		backend := b.backends[i%len(b.backends)]

		if backend.Alive.Load() {
			b.rw.RUnlock()
			return backend, nil
		}

		b.rw.Unlock()
	}

	return nil, errors.New("no available backend")
}

func (b *Balancer) AddBackend(backend *Backend) {
	if !backend.Alive.Load() {
		return
	}

	b.rw.Lock()
	defer b.rw.Unlock()
	b.backends = append(b.backends, backend)
	b.backsIndexes[backend] = len(b.backends) - 1
}

func (b *Balancer) RemoveBackend(backend *Backend) {
	b.rw.Lock()
	defer b.rw.Unlock()

	i, ok := b.backsIndexes[backend]
	if !ok {
		return
	}

	b.backends[i] = b.backends[len(b.backends)-1]
	b.backsIndexes[b.backends[i]] = i

	b.backends = b.backends[:len(b.backends)-1]
	delete(b.backsIndexes, backend)

}
