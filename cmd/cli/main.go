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
		return
	}

	command := os.Args[1]

	switch command {
	case "server":
		core.ServerMain()
	case "client":
		//if the args list contains "--bind" pass it to ClientMain
		bind := false
		if len(os.Args) > 2 && strings.ToLower(os.Args[2]) == "--bind" {
			bind = true
		}
		core.ClientMain(bind)
	default:
		return
	}
}
