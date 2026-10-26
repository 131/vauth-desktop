//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "vauth-desktop: the window is only available on Windows (WebView2).")
	os.Exit(1)
}
