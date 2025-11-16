package command

import (
	"context"
	"fmt"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/ui/diff"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

// PlanCmd shows configuration differences without applying them.
func PlanCmd(ctx context.Context) *cobra.Command {
	var statePath string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show configuration differences without applying changes",
		Long: `Compares the desired ldappy configuration file with the current system state.
Displays a human-readable diff or a JSON patch (when --json is enabled).`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			cfgPath, _ := c.Root().PersistentFlags().GetString("config")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.Section("Configuration Plan")

			log.InfoCtx(ctx, "Loading desired configuration from %s", cfgPath)
			desired, derr := config.Load(cfgPath)
			if derr != nil {
				log.ErrorCtx(ctx, "Failed to load desired configuration: %v", derr)
				return fmt.Errorf("failed to load config: %w", derr)
			}

			log.InfoCtx(ctx, "Loading current state from %s", statePath)
			current, _ := config.Load(statePath)

			diffMode := diff.HumanReadable
			if jsonOutput {
				diffMode = diff.JSONPatch
			}

			d := diff.Diff(current, desired, diffMode)

			if d == "" {
				log.SuccessCtx(ctx, "No configuration changes detected.")
				log.SectionEnd()
				return nil
			}

			if jsonOutput {
				fmt.Println(d)
			} else {
				log.WarnCtx(ctx, "Proposed configuration changes detected:")
				fmt.Println("\n──────── CHANGE PLAN ────────")
				fmt.Println(d)
				fmt.Println("─────────────────────────────")
			}

			log.SectionEnd()
			return nil
		},
	}

	cmd.Flags().StringVar(&statePath, "state", "/var/lib/openldap-setup/state.toml", "Path to current state file")

	return cmd
}
