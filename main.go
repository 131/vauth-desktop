package main

import "log"

func main() {
	if err := NewRuntime(NewApp(nil)).Run(); err != nil {
		log.Fatal(err)
	}
}
