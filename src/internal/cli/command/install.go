package command

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/kitechsoftware/ldappy/internal/cli/lifecycle"
	"github.com/kitechsoftware/ldappy/internal/common/core"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

func InstallCmd(ctx context.Context) *cobra.Command {
	var (
		isContainer bool
		printCreds  bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install OpenLDAP using the provided configuration",
		Long: `Installs OpenLDAP (slapd) and ldap-utils packages, 
seeds the debconf configuration, and enables the service automatically.`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			cfgPath, _ := c.Root().PersistentFlags().GetString("config")

			log.SetJSONOutput(jsonOutput)
			c.SilenceUsage = true
			// Check for cancellation before heavy work
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if os.Geteuid() != 0 {
				return fmt.Errorf("❌ insufficient privileges — please run as root or with sudo (use --force to override)")
			}

			log.InfoCtx(ctx, "Loading configuration from %s", cfgPath)
			cfg, err := core.Load(ctx, cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log.InfoCtx(ctx, "Starting OpenLDAP installation (container=%v)", isContainer)
			report := lifecycle.Install(ctx, cfg, isContainer, jsonOutput)

			// Print and short-circuit if cancelled mid-run
			if ctx.Err() != nil {
				log.WarnCtx(ctx, "Installation interrupted: %v", ctx.Err())
				return ctx.Err()
			}

			report.Finish(jsonOutput)

			if !report.Success {
				return fmt.Errorf("install failed: %s", report.ErrorMsg)
			}

			if !jsonOutput && printCreds {
				color.Green("🎉 OpenLDAP installation complete.")
				if printCreds {
					color.Yellow("Admin DN: cn=%s,%s", cfg.LDAP.AdminUser, cfg.LDAP.BaseDN)
					color.Yellow("Password : %s", cfg.LDAP.AdminPassword)
				}
			}

			log.SuccessCtx(ctx, "Installation finished successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&isContainer, "container", false, "Install in a container environment")
	cmd.Flags().BoolVar(&printCreds, "print-credentials", false, "Display generated admin credentials after install")

	return cmd
}
