package command

import (
	"context"
	"fmt"
	"os"

	"github.com/kitechsoftware/ldappy/internal/cli/lifecycle"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

func UpgradeCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade OpenLDAP to the latest version safely",
		Long: `Upgrade OpenLDAP (slapd and ldap-utils) to the latest available
packages from the system's package manager. Automatically restarts slapd
after upgrade.`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if os.Geteuid() != 0 {
				return fmt.Errorf("❌ insufficient privileges — please run as root or with sudo (use --force to override)")
			}

			log.Section("Upgrade OpenLDAP")
			log.InfoCtx(ctx, "Starting upgrade of slapd and ldap-utils...")

			report := lifecycle.Upgrade(ctx, jsonOutput)
			report.Finish(jsonOutput)

			if !report.Success {
				log.ErrorCtx(ctx, "Upgrade failed: %s", report.ErrorMsg)
				return fmt.Errorf("upgrade failed: %s", report.ErrorMsg)
			}

			if !jsonOutput {
				log.SuccessCtx(ctx, "🎉 OpenLDAP upgrade completed successfully.")
			}

			log.SectionEnd()
			return nil
		},
	}

	return cmd
}
