package main

import (
	"sync"
	"testing"
)

func TestOpenWindowCoalescesReentrantBursts(t *testing.T) {
	r := &Runtime{}
	created, shown := 0, 0
	var open func()
	burst := func() {
		var wg sync.WaitGroup
		for range 14 {
			wg.Go(open)
		}
		wg.Wait()
	}
	create := func() *WindowSession {
		created++
		open()  // Same-thread reentrancy from the native message pump.
		burst() // WebView initialization can dispatch more tray clicks.
		return &WindowSession{}
	}
	show := func(s *WindowSession) {
		shown++
		if r.session() != s {
			t.Fatal("showing an unpublished session")
		}
		open()
		burst() // Restore/Show/Focus can also dispatch native messages.
	}
	open = func() { r.openWindowWith(create, show) }
	open()
	first := r.session()
	if created != 1 || shown != 1 || first == nil {
		t.Fatalf("initial burst: created=%d shown=%d session=%p", created, shown, first)
	}
	// Existing or hidden sessions must be reused on every subsequent click.
	for range 14 {
		open()
	}
	if created != 1 || shown != 15 || r.session() != first {
		t.Fatalf("existing session replaced: created=%d shown=%d", created, shown)
	}
	first.closed.Store(true)
	r.closed(first)
	open()
	if created != 2 || shown != 16 || r.session() == first || r.session() == nil {
		t.Fatalf("reopen burst: created=%d shown=%d", created, shown)
	}
	r.closed(first)
	if r.session() == nil {
		t.Fatal("old close callback cleared replacement session")
	}
}

func TestOpenWindowDoesNotPublishClosedConstruction(t *testing.T) {
	r := &Runtime{}
	s := &WindowSession{}
	r.openWindowWith(func() *WindowSession {
		s.closed.Store(true)
		r.closed(s)
		return s
	}, func(*WindowSession) {})
	if r.session() != nil || r.opening {
		t.Fatal("closed construction was published or reservation was retained")
	}
}
