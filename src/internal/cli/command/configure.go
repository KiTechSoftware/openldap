package command

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/kitechsoftware/ldappy/internal/cli/configure"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

// ConfigureCmd defines the top-level `ldappy configure` command group.
func ConfigureCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Perform administrative configuration tasks for OpenLDAP",
		Long: `Run administrative configuration tasks on OpenLDAP.

Examples:
  ldappy configure hash "MySecret123"
  ldappy configure reset-admin "NewPass!"
  ldappy configure apply --file /tmp/patch.ldif
  ldappy configure backup
  ldappy configure rollback --file /var/backups/ldappy/ldap-backup.ldif --json`,
	}

	cmd.AddCommand(
		configureHashCmd(ctx),
		configureResetAdminCmd(ctx),
		configureApplyCmd(ctx),
		configureBackupCmd(ctx),
		configureRollbackCmd(ctx),
	)

	return cmd
}

// ---------- Subcommands ----------

// `ldappy configure hash "password"`
func configureHashCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hash [password]",
		Short: "Generate an LDAP-compatible password hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			hash, err := configure.HashPassword(ctx, args[0])
			if err != nil {
				log.ErrorCtx(ctx, "Password hashing failed: %v", err)
				return err
			}

			if jsonOutput {
				fmt.Printf("{\"hash\": %q}\n", hash)
			} else {
				color.Green("🔐 Generated LDAP password hash:\n%s", hash)
			}
			return nil
		},
	}
	return cmd
}

// `ldappy configure reset-admin "newpassword"`
func configureResetAdminCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-admin [new_password]",
		Short: "Reset the OpenLDAP admin (rootDN) password",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.InfoCtx(ctx, "Resetting admin password...")
			report := configure.ResetAdminPassword(ctx, args[0], jsonOutput)
			log.InfoCtx(ctx, "Reset operation finished.")

			if !report.Success {
				log.ErrorCtx(ctx, "Reset failed: %s", report.ErrorMsg)
				return fmt.Errorf("reset failed: %s", report.ErrorMsg)
			}

			log.SuccessCtx(ctx, "Admin password reset successfully.")
			return nil
		},
	}
	return cmd
}

// `ldappy configure apply --file file.ldif`
func configureApplyCmd(ctx context.Context) *cobra.Command {
	var file string
	var interactive bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply an LDIF configuration patch or reconfigure interactively",
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.InfoCtx(ctx, "Applying LDIF configuration from %s", file)
			report := configure.ApplyLDIF(ctx, file, interactive, jsonOutput)
			if !report.Success {
				log.ErrorCtx(ctx, "Apply failed: %s", report.ErrorMsg)
				return fmt.Errorf("apply failed: %s", report.ErrorMsg)
			}
			log.SuccessCtx(ctx, "LDIF applied successfully.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to LDIF file to apply")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run interactive dpkg-reconfigure")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

// `ldappy configure backup`
func configureBackupCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup the current OpenLDAP directory to /var/backups/ldappy",
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.InfoCtx(ctx, "Starting OpenLDAP backup...")
			report := configure.Backup(ctx, jsonOutput)
			if !report.Success {
				log.ErrorCtx(ctx, "Backup failed: %s", report.ErrorMsg)
				return fmt.Errorf("backup failed: %s", report.ErrorMsg)
			}

			log.SuccessCtx(ctx, "Backup completed successfully.")
			return nil
		},
	}
	return cmd
}

// `ldappy configure rollback --file backup.ldif`
func configureRollbackCmd(ctx context.Context) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore OpenLDAP data from a backup LDIF file",
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.InfoCtx(ctx, "Restoring backup from %s", file)
			report := configure.Rollback(ctx, file, jsonOutput)
			if !report.Success {
				log.ErrorCtx(ctx, "Rollback failed: %s", report.ErrorMsg)
				return fmt.Errorf("rollback failed: %s", report.ErrorMsg)
			}

			log.SuccessCtx(ctx, "Rollback completed successfully.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to LDIF backup file to restore from")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}
