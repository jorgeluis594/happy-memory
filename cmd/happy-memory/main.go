// Command happy-memory is the entry point for the happy-memory CLI.
package main

import (
	"context"
	"os"

	"github.com/jorgeluis594/happy-memory/internal/app"
)

func main() {
	os.Exit(app.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
