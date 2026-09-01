//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("Tavern Shelf desktop is currently available for Windows; use cmd/tavern-shelf for headless mode.")
}
