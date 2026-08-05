// Command happy-memory is the entry point for the happy-memory CLI.
package main

import "github.com/spf13/cobra"

func main() {
	rootCommand := &cobra.Command{
		Use:   "happy-memory",
		Short: "Happy Memory command-line interface",
	}

	cobra.CheckErr(rootCommand.Execute())
}
