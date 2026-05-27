package crawler

import (
	"fmt"
	"sync"
	"testing"
)

var exampleLink, _ = NewLink("link-1")

func TestFrontier(t *testing.T) {
	t.Run("should push and pop", func(t *testing.T) {
		f := NewFrontier()
		want := exampleLink
		f.Push(want)

		got := f.Pop()

		if *got != want {
			t.Fatalf("got %v, but wanted %v", got, want)
			return
		}
	})

	t.Run("popped should be removed", func(t *testing.T) {
		f := NewFrontier()
		want := exampleLink
		f.Push(want)
		f.Pop()

		if f.Len() != 0 {
			t.Fatalf("got %v, but wanted %v", f.Len(), 0)
		}
	})

	t.Run("peek shouldn't remove", func(t *testing.T) {
		f := NewFrontier()
		want := exampleLink
		f.Push(want)
		got := f.Peek()

		if *got != want {
			t.Fatalf("got %v, but wanted %v", got, want)
		}

		if f.Len() != 1 {
			t.Fatalf("got %v, but wanted %v", f.Len(), 1)
		}
	})

	t.Run("pop elegible should return if match", func(t *testing.T) {
		f := NewFrontier()
		f.Push(exampleLink)

		isElegible := func(link Link) bool {
			return link.Normalized == exampleLink.Normalized
		}
		got := f.PopElegible(isElegible)
		if got == nil {
			t.Fatal("expected elegible link, but got nil")
		}

		if *got != exampleLink {
			t.Fatalf("got %v, but wanted %v", got, exampleLink)
		}
	})
}

func TestFrontierNoDuplicates(t *testing.T) {
	f := NewFrontier()

	links := []Link{
		exampleLink, exampleLink,
	}
	for _, link := range links {
		f.Push(link)
	}

	got := f.Len()
	want := 1

	if got != want {
		t.Fatalf("got %v, but wanted %v", got, want)
	}
}

func TestFrontierShouldSync(t *testing.T) {
	f := NewFrontier()

	wg := sync.WaitGroup{}
	for i := range 10 {
		wg.Add(1)
		wg.Go(func() {
			defer wg.Done()
			newLink, _ := NewLink(fmt.Sprintf("link-%d", i))
			f.Push(newLink)
		})
	}
	wg.Wait()
}
