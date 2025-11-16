package command

import (
	"context"
	"fmt"

	"github.com/kitechsoftware/ldappy/internal/cli/status"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"

	"github.com/spf13/cobra"
)

// VersionCmd prints the current version of ldappy and OpenLDAP components.
func VersionCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show ldappy and OpenLDAP version information",
		Long: `Displays the version of ldappy along with the underlying OpenLDAP (slapd)
version if available. Supports JSON output for automation.`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			v := status.Version

			if jsonOutput {
				// Structured machine-readable output
				fmt.Printf("{\"version\": %q}\n", v)
				return nil
			}

			log.Section("ldappy Version")
			log.SuccessCtx(ctx, "ldappy version: %s", v)
			log.SectionEnd()

			return nil
		},
	}
	return cmd
}
