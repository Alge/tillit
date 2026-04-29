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
	case "forget":
		err = commands.Forget(args)
	case "peers":
		err = commands.TrustList(args)
	case "sign":
		err = commands.Sign(args)
	case "revoke":
		err = commands.Revoke(args)
	case "delete":
		err = commands.Delete(args)
	case "sync":
		err = commands.Sync(args)
	case "publish":
		err = commands.Publish(args)
	case "mirror":
		err = commands.Mirror(args)
	case "export":
		err = commands.Export(args)
	case "import":
		err = commands.Import(args)
	case "status":
		err = commands.Status(args)
	case "query":
		err = commands.Query(args)
	case "inspect":
		err = commands.Inspect(args)
	case "check":
		err = commands.Check(args)
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
  trust <id@url> [--depth N] [--public] [--veto-only]
                              add or update a trusted peer
  distrust <id>               explicitly distrust a peer (blocks transitive trust)
  forget <id>                 remove a peer entirely (revokes any published trust connection)
  peers                       list all configured peers
  sign version <eco> <pkg> <version> --level <l> [--reason "..."]
                              sign a vetting decision for an exact version
  sign delta <eco> <pkg> <from> <to> --level <l> [--reason "..."]
                              sign review of the changes between two versions
  revoke <signature_id>       revoke a previously published decision
  delete <signature_id>       remove a locally-cached signature that has not yet been pushed
                              (use revoke instead once a signature is on a server)
  sync                        pull signatures from all trusted peers into local cache
  publish                     push any locally-cached signatures to registered servers
  mirror push <server>        privately push your sigs+connections to your own server (cross-device backup)
  mirror pull <server>        pull your private sigs+connections back from that server
  export [--all | --key <name>] [--include-peers] <file>
                              write a snapshot (incl. private key) to a file
                              default: active key + own data
                              --key <name>: a non-active key + its own data
                              --all: every key + every row (full backup; conflicts with --key)
                              --include-peers: also rows by signers in the identity's trust graph
  import <file>               merge a previously-exported snapshot into the local store
  status                      show pending pushes and last-sync time per registered server
  query <ecosystem> <pkg> [--verbose]
                              show trusted versions of a package, grouped by status
  inspect <signature_id>      show full details of a cached signature (accepts a hash prefix)
  check [-e <ecosystem>] [path]
                              check every package against the trust graph
                              (path defaults to '.'; ecosystem required until .tillit lands)`)
}
