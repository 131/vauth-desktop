package main

import (
	"encoding/json"
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Uses a dedicated Wails app and synthetic HTML: no Vault, agent or installed app.
func TestNativeSessionBridge(t *testing.T) {
	if os.Getenv("VAUTH_DESKTOP_NATIVE_TESTS") == "" {
		t.Skip("native bridge opt-in")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := windows.NewLazySystemDLL("ole32.dll")
	initialized, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if initialized != 0 && initialized != 1 {
		t.Fatalf("COM initialization: %#x", initialized)
	}
	defer ole.NewProc("CoUninitialize").Call()
	result := make(chan error, 1)
	var once sync.Once
	var app *application.App
	var session *WindowSession
	app = application.New(application.Options{Name: "vauth-native-bridge-test", RawMessageHandler: func(window application.Window, message string, _ *application.OriginInfo) {
		if strings.HasPrefix(message, "probe:") {
			once.Do(func() {
				var reply struct {
					ID      int
					Page    string
					Result  string
					Error   string
					Expired bool
				}
				err := json.Unmarshal([]byte(strings.TrimPrefix(message, "probe:")), &reply)
				if err == nil && (reply.ID != 73 || reply.Page != "native-page" || reply.Result != version || reply.Error != "" || reply.Expired) {
					err = fmt.Errorf("bad bridge response: %+v", reply)
				}
				result <- err
				application.InvokeAsync(app.Quit)
			})
			return
		}
		if session != nil && session.view.ID() == window.ID() {
			session.Message(message)
		}
	}})
	session = NewWindowSession(&App{}, app, func(*WindowSession) {})
	session.view.SetHTML(`<html><body>synthetic bridge<script>
 window.vauthDone=reply=>window._wails.invoke('probe:'+JSON.stringify(reply));
 window.addEventListener('wails:runtime-config-ready',()=>{
  window._wails.invoke('wails:runtime:ready');
  window._wails.invoke(JSON.stringify({id:73,page:'native-page',method:'version',args:[]}));
 });
 </script></body></html>`)
	session.Show()
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-time.After(90 * time.Second):
			once.Do(func() { result <- fmt.Errorf("native bridge timed out"); application.InvokeAsync(app.Quit) })
		}
	}()
	err := app.Run()
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("app quit before bridge reply")
	}
}
