package wait

import (
	"context"
	"sync"
)

type Group struct {
	wg sync.WaitGroup
}

func (g *Group) Add(f func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		f()
	}()
}
func (g *Group) AddMany(count int, f func()) {
	for i := 0; i < count; i++ {
		g.Add(f)
	}
}

func (g *Group) Wait(ctx context.Context) {
	stopCh := make(chan struct{})
	go func() {
		g.wg.Wait()
		stopCh <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-stopCh:
	}

	return
}
