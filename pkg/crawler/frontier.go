package crawler

import "sync"

type Frontier struct {
	queue []Link
	seen  map[string]struct{}
	mu    sync.Mutex
}

func NewFrontier() *Frontier {
	return &Frontier{
		queue: []Link{},
		seen:  map[string]struct{}{},
	}
}

func (f *Frontier) Push(link Link) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.seen[link.Normalized]; ok {
		return false
	}
	f.queue = append(f.queue, link)
	f.seen[link.Normalized] = struct{}{}
	return true
}

func (f *Frontier) Pop() *Link {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queue) < 1 {
		return nil
	}
	first := f.queue[0]
	f.queue = f.queue[1:]

	return &first
}

func (f *Frontier) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queue)
}

func (f *Frontier) Peek() *Link {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queue) < 1 {
		return nil
	}
	first := f.queue[0]
	return &first
}

func (f *Frontier) PopElegible(isElegible func(Link) bool) *Link {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.queue) < 1 {
		return nil
	}
	for i, link := range f.queue {
		if isElegible(link) {
			f.queue = append(f.queue[:i], f.queue[i+1:]...)
			return &link
		}
	}
	return nil
}
