package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/kitechsoftware/ldappy/internal/cli/command"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	verbose    bool
	cfgPath    string
)

func main() {
	// Handle SIGINT/SIGTERM (Ctrl-C or kill)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		color.New(color.FgRed, color.Bold).Fprintln(os.Stderr, "Error:", err)

		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	root := &cobra.Command{
		Use:   "ldappy",
		Short: "Declarative OpenLDAP installer and manager",
		Long:  `State-aware, transactional OpenLDAP installer with plan/apply/verify/rollback.`,
		PersistentPreRunE: func(command *cobra.Command, args []string) error {
			if os.Getenv("LDAPPY_DEBUG") != "" || verbose {
				log.EnableDebug()
				log.Debug("Verbose logging enabled")
			}
			return nil
		},
	}

	root.SilenceErrors = true
	root.SilenceUsage = true

	root.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output results in JSON format")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose debug logging")
	root.PersistentFlags().StringVarP(&cfgPath, "config", "C", "", "Path to configuration file")
	// Pass context into all subcommands
	root.AddCommand(
		command.PurgeCmd(ctx),
		command.InstallCmd(ctx),
		command.InitCmd(ctx),
		command.VerifyCmd(ctx),
		command.VersionCmd(ctx),
		command.UpgradeCmd(ctx),
		command.ConfigureCmd(ctx),
		command.ServiceCmd(ctx),
		command.StatusCmd(ctx),
	)

	return root.ExecuteContext(ctx)
}
