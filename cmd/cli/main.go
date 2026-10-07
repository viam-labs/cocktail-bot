package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.viam.com/rdk/cli"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
)

var rootCmd = &cobra.Command{
	Use:          "cocktail-cli",
	Short:        "Developer CLI for the cocktail bot",
	Long:         "Developer CLI for testing cocktail-bot behaviours against a live machine. Authenticates with cached `viam login` credentials.",
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(hoverGlassCmd)
	rootCmd.AddCommand(pourGlassCmd)
}

func dialMachine(ctx context.Context, address string, logger logging.Logger) (robot.Robot, error) {
	machine, err := cli.ConnectToMachine(ctx, address, logger)
	if err != nil {
		return nil, fmt.Errorf("dial machine: %w", err)
	}
	return machine, nil
}

func newLogger(name string) logging.Logger {
	logger := logging.NewBlankLogger(name)
	logger.SetLevel(logging.INFO)
	logger.AddAppender(logging.NewWriterAppender(os.Stderr))
	return logger
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
