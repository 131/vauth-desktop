package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/131/vauth-desktop/v2/internal/vault"
)

func TestBridgePreservesPageAndExpiry(t *testing.T) {
	for _, err := range []error{nil, errors.New("ordinary error"), vault.ErrSessionExpired} {
		call := bridgeCall{ID: 42, Page: "first-page"}
		var reply struct {
			ID      int
			Page    string
			Error   string
			Expired bool
			Result  string
		}
		if e := json.Unmarshal(bridgeResponse(call, "synthetic", err), &reply); e != nil {
			t.Fatal(e)
		}
		if reply.ID != 42 || reply.Page != "first-page" || reply.Result != "synthetic" || reply.Expired != errors.Is(err, vault.ErrSessionExpired) {
			t.Fatalf("wrong reply: %+v", reply)
		}
	}
}
func TestClosedSessionDropsCallsAndReplies(t *testing.T) {
	calls := 0
	s := &WindowSession{app: App{Copy: func(string) error { calls++; return nil }}}
	s.closed.Store(true)
	s.Message(`{"id":1,"page":"old","method":"copy","args":["synthetic"]}`)
	s.eval("old reply")
	if calls != 0 {
		t.Fatal("closed session executed a new call")
	}
}

func TestClosedSessionRejectsCaptionAction(t *testing.T) {
	s := &WindowSession{}
	s.closed.Store(true)
	if err := s.WindowAction("maximize"); err == nil {
		t.Fatal("closed caption action accepted")
	}
}
