package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("otelkit - local observability CLI")
		fmt.Println("usage: otelkit <command>")
		fmt.Println("commands: run, dash, version")
		return
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("otelkit dev")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}
