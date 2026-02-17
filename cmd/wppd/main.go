package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/matheus3301/wpp/internal/daemon"
	"github.com/matheus3301/wpp/internal/session"
	"go.uber.org/fx"
)

func main() {
	sessionFlag := flag.String("session", "", "session name (overrides config default)")
	flag.Parse()

	sessionName := session.Resolve(*sessionFlag)
	if err := session.ValidateName(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	app := fx.New(
		daemon.Module(daemon.Params{SessionName: sessionName}),
	)

	app.Run()
}
