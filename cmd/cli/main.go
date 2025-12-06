package cli

import (
	"fmt"
	"os"

	"github.com/notassigned/endershare/internal/core"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: endershare <command> [arguments]")
		return
	}

	command := os.Args[1]

	switch command {
	case "server":
		core.ServerMain()
	case "client":
		core.ClientMain()
	default:
		return
	}
}
