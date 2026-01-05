package main

import (
	"fmt"
	"os"

	"github.com/mengbin92/blog/cmd/blog/commands"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "blog",
	Short: "Blog Static Site Generator",
	Long: `A fast and flexible static site generator for blogs.
Supports markdown posts, customizable themes, and static asset management.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default command is build
		commands.BuildCmd.Run(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(commands.BuildCmd)
	rootCmd.AddCommand(commands.VersionCmd)
	rootCmd.AddCommand(commands.InitCmd)
}
