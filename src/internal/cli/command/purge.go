package command

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/kitechsoftware/ldappy/internal/cli/lifecycle"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

func PurgeCmd(ctx context.Context) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Completely remove OpenLDAP and all related data",
		Long: `This command stops slapd, removes packages, configuration,
and data directories. It is irreversible unless backups exist.
Use --force to skip confirmation or --json for structured output.`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)
			c.SilenceUsage = true

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.Section("Purge OpenLDAP Installation")

			if !force && os.Geteuid() != 0 {
				return fmt.Errorf("❌ insufficient privileges — please run as root or with sudo (use --force to override)")
			}

			if !force && !jsonOutput {
				reader := bufio.NewReader(os.Stdin)
				color.Yellow("⚠️  This will completely remove OpenLDAP and all its data.")
				fmt.Print("Are you sure you want to continue? (y/N): ")
				resp, _ := reader.ReadString('\n')
				if strings.ToLower(strings.TrimSpace(resp)) != "y" {
					log.WarnCtx(ctx, "User aborted purge.")
					return nil
				}
			}

			log.InfoCtx(ctx, "Starting purge operation...")
			start := time.Now()

			report := lifecycle.Purge(ctx, jsonOutput)
			report.Finish(jsonOutput)

			if !report.Success {
				return fmt.Errorf("purge failed: %s", report.ErrorMsg)
			}

			if !jsonOutput {
				color.Green("🎉 OpenLDAP fully purged successfully.")
			}

			log.Timed("Purge operation", start, report.Success, nil)
			log.SectionEnd()

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
