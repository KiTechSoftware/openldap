package command

import (
	"context"
	"fmt"

	"github.com/kitechsoftware/ldappy/internal/cli/verify"
	"github.com/kitechsoftware/ldappy/internal/common/core"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

func VerifyCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run post-installation or configuration verification checks",
		Long: `Performs multiple health and consistency checks on OpenLDAP after installation or configuration changes.
These include service state, configuration validity, schema correctness, and network accessibility.`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			cfgPath, _ := c.Root().PersistentFlags().GetString("config")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			log.Section("Verification Checks")

			log.InfoCtx(ctx, "Loading configuration from %s", cfgPath)
			cfg, err := core.Load(ctx, cfgPath)
			if err != nil {
				log.ErrorCtx(ctx, "Failed to load configuration: %v", err)
				return fmt.Errorf("failed to load config: %w", err)
			}

			log.InfoCtx(ctx, "Running OpenLDAP verification suite...")
			report := verify.RunAll(ctx, cfg)

			verify.PrintSummary(report, jsonOutput)

			if !report.AllSuccessful {
				log.ErrorCtx(ctx, "One or more verification checks failed.")
				log.SectionEnd()
				return fmt.Errorf("verification failed: one or more checks failed")
			}

			if !jsonOutput {
				log.SuccessCtx(ctx, "✅ All verification checks passed successfully.")
			}

			log.SectionEnd()
			return nil
		},
	}

	return cmd
}
