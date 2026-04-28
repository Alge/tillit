package main

import (
	"fmt"
	"os"

	"github.com/Alge/tillit/cmd/tillit/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = commands.Init(args)
	case "key":
		err = commands.Key(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `usage: tillit <command> [args]

commands:
  init              initialize local tillit storage and generate a default key
  key generate      generate a new named key
  key list          list all stored keys
  key show <name>   show the public key for a named key
  key use <name>    set the active key`)
}
