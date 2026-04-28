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
	case "register":
		err = commands.Register(args)
	case "trust":
		err = commands.Trust(args)
	case "distrust":
		err = commands.Distrust(args)
	case "untrust":
		err = commands.Untrust(args)
	case "peers":
		err = commands.TrustList(args)
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
  init                        initialize local tillit storage and generate a default key
  key generate <name> [alg]   generate a new named key
  key list                    list all stored keys
  key show <name>             show the public key for a named key
  key use <name>              set the active key
  register <url> [alias]      register active key on a server
  trust <id@url> [--depth N] [--delegate]
                              add or update a trusted peer
  distrust <id@url>           explicitly distrust a peer (blocks transitive trust)
  untrust <id@url>            remove a peer entirely
  peers                       list all configured peers`)
}
