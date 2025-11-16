package command

import (
	"context"
	"fmt"
	"time"

	"github.com/kitechsoftware/ldappy/internal/cli/lifecycle"
	"github.com/kitechsoftware/ldappy/internal/common/core"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
	"github.com/spf13/cobra"
)

func InitCmd(ctx context.Context) *cobra.Command {
	var (
		printCreds bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the LDAP Base DN and admin entry",
		Long: `Initializes the LDAP directory with a base DN, organization,
and admin entry based on your configuration file.
If the directory is already initialized, the operation is skipped unless --force is used.`,
		RunE: func(c *cobra.Command, args []string) error {
			start := time.Now()
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			cfgPath, _ := c.Root().PersistentFlags().GetString("config")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.Section("Initialize OpenLDAP Directory")

			cfg, err := core.Load(ctx, cfgPath)
			if err != nil {
				log.ErrorCtx(ctx, "Failed to load configuration: %v", err)
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			log.InfoCtx(ctx, "Starting LDAP initialization (force=%v)", force)
			report := lifecycle.Init(ctx, cfg, jsonOutput, force)

			// If context was cancelled mid-operation
			if ctx.Err() != nil {
				log.WarnCtx(ctx, "Initialization interrupted: %v", ctx.Err())
				return ctx.Err()
			}

			report.Finish(jsonOutput)
			log.Timed("LDAP initialization", start, report.Success, err)

			if !report.Success {
				return fmt.Errorf("initialization failed: %s", report.ErrorMsg)
			}

			if !jsonOutput {
				log.SuccessCtx(ctx, "🎉 Base DN initialized successfully.")
				if printCreds {
					log.WarnCtx(ctx, "Admin DN: cn=%s,%s", cfg.LDAP.AdminUser, cfg.LDAP.BaseDN)
					log.WarnCtx(ctx, "Password : %s", cfg.LDAP.AdminPassword)
				}
				log.SectionEnd()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&printCreds, "print-credentials", false, "Display admin credentials if initialization succeeds")
	cmd.Flags().BoolVar(&force, "force", false, "Force reinitialization even if slapd is already configured")

	return cmd
}
