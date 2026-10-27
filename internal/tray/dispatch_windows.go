package tray

import "sync"

type uiDispatch struct {
	mu    sync.Mutex
	queue []func()
}

// Dispatch targets the HWND, so native modal loops deliver these callbacks.
func (t *Tray) Dispatch(f func()) {
	t.dispatch.mu.Lock()
	t.dispatch.queue = append(t.dispatch.queue, f)
	t.dispatch.mu.Unlock()
	procPostMessage.Call(t.hwnd, wmDispatch, 0, 0)
}

func (t *Tray) runDispatched() {
	t.dispatch.mu.Lock()
	queue := t.dispatch.queue
	t.dispatch.queue = nil
	t.dispatch.mu.Unlock()
	for _, f := range queue {
		f()
	}
}
