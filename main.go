package main

import (
	"fmt"
	"os"
	"os/exec"
)

// main.go at root is a convenience wrapper for running cmd/pulse
// in production, use the binary built from cmd/pulse directly
func main() {
	cmd := exec.Command("go", "run", "./cmd/pulse")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
