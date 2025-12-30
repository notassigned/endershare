package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/notassigned/endershare/internal/core"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: endershare <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  peer          Start a replica node (joins existing network)")
		fmt.Println("  peer --init   Initialize a new master node")
		fmt.Println("  bind <phrase> Authorize a new peer (master nodes only)")
		return
	}

	command := os.Args[1]

	switch command {
	case "peer":
		// Check for --init flag
		initMode := false
		if len(os.Args) > 2 && strings.ToLower(os.Args[2]) == "--init" {
			initMode = true
		}
		core.PeerMain(initMode)

	case "bind":
		if len(os.Args) < 3 {
			fmt.Println("Error: bind command requires a sync phrase")
			fmt.Println("Usage: endershare bind <sync-phrase>")
			os.Exit(1)
		}
		// Join all remaining args as the sync phrase (in case it has spaces)
		syncPhrase := strings.Join(os.Args[2:], " ")
		core.BindMain(syncPhrase)

	default:
		fmt.Println("Unknown command:", command)
		fmt.Println("Run 'endershare' for usage information")
		os.Exit(1)
	}
}
