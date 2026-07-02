package main

import (
	"fmt"
	"os"

	"github.com/mengbin92/gobin/cmd/gobin/commands"
	"github.com/mengbin92/gobin/internal/config"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gobin",
	Short: "Blog Static Site Generator",
	Long: `A fast and flexible static site generator for blogs.
Supports markdown posts, customizable themes, and static asset management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.RunDefaultBuild(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(commands.BuildCmd)
	rootCmd.AddCommand(commands.VersionCmd)
	rootCmd.AddCommand(commands.InitCmd)
	rootCmd.AddCommand(commands.ServeCmd)
	rootCmd.AddCommand(commands.NewCmd)
	rootCmd.AddCommand(commands.CheckCmd)

	commands.AddGlobalFlags(rootCmd)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cfg, _ := config.LoadIfPresent()
		if err := commands.InitLogging(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
