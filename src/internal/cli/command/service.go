package command

import (
	"context"
	"fmt"

	"github.com/kitechsoftware/ldappy/internal/cli/lifecycle"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

// ServiceCmd defines `ldappy service` command group.
func ServiceCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage OpenLDAP service (start, stop, restart, status)",
		Long: `Control the OpenLDAP (slapd) service lifecycle.
Examples:
  ldappy service start
  ldappy service status
  ldappy service restart --json`,
	}

	cmd.AddCommand(
		serviceSubCmd(ctx, "start"),
		serviceSubCmd(ctx, "stop"),
		serviceSubCmd(ctx, "restart"),
		serviceSubCmd(ctx, "status"),
	)

	return cmd
}

func serviceSubCmd(ctx context.Context, action string) *cobra.Command {
	var (
		isContainer bool
		daemon      bool
	)
	cmd := &cobra.Command{
		Use:   action,
		Short: fmt.Sprintf("%s the slapd service", action),
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			// Respect cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.Section(fmt.Sprintf("Service: %s", action))
			log.InfoCtx(ctx, "Performing systemctl %s slapd...", action)

			report := lifecycle.Service(ctx, action, jsonOutput, isContainer, daemon)
			report.Finish(jsonOutput)

			if !report.Success {
				log.ErrorCtx(ctx, "Service %s failed: %s", action, report.ErrorMsg)
				return fmt.Errorf("service %s failed: %s", action, report.ErrorMsg)
			}

			if !jsonOutput {
				log.SuccessCtx(ctx, "slapd %sed successfully.", action)
			}

			log.SectionEnd()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&isContainer, "container", "c", false, "Run in a container environment")
	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run as a daemon")

	return cmd
}
