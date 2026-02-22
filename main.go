package main

import (
	"fmt"
	"os"
)

var (
	// set by -ldflags at build time
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "hello":
		fmt.Println("necro: hello")
	case "version":
		fmt.Printf("necro %s (commit=%s, date=%s)\n", version, commit, date)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("necro - multi-account AWS helper CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  necro <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  hello     print hello")
	fmt.Println("  version   print build info")
	fmt.Println("  help      show this help")
}
