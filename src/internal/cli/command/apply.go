package command

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/kitechsoftware/ldappy/internal/cli/configure"
	"github.com/kitechsoftware/ldappy/internal/cli/lifecycle"
	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/kitechsoftware/ldappy/internal/common/ui/diff"

	"github.com/spf13/cobra"
)

type DiffItem struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

func ApplyCmd(ctx context.Context) *cobra.Command {
	var cfgPath, statePath string
	var autoApprove, jsonOutput, isContainer bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply configuration and update system state (install, init, and configure OpenLDAP)",
		RunE: func(c *cobra.Command, args []string) error {
			desired, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			current, _ := config.Load(statePath)

			// Compute diff
			diffMode := diff.HumanReadable
			if jsonOutput {
				diffMode = diff.JSONPatch
			}
			dif := diff.Diff(current, desired, diffMode)

			// Optional: parse JSON diff for structured output
			var structuredDiff []DiffItem
			if jsonOutput && dif != "" && dif != "[]" {
				_ = json.Unmarshal([]byte(dif), &structuredDiff)
			}

			if dif == "" || dif == "[]" {
				color.New(color.FgGreen).Println("No changes to apply.")
				return nil
			}

			if jsonOutput {
				fmt.Println(dif)
			} else {
				fmt.Println(color.HiYellowString("\n──────── CHANGE PLAN ────────"))
				fmt.Println(dif)
				fmt.Println(color.HiYellowString("────────────────────────────\n"))
			}

			if !autoApprove {
				fmt.Print("Apply these changes? [Y/n]: ")
				in := bufio.NewReader(os.Stdin)
				ans, _ := in.ReadString('\n')
				if ans != "" && (ans[0] == 'n' || ans[0] == 'N') {
					color.Yellow("Aborted by user.")
					return nil
				}
			}

			// Snapshot for rollback
			snap, _ := configure.Create("pre-apply")
			defer func() {
				if r := recover(); r != nil {
					color.Red("❌ Apply crashed — restoring previous state...")
					_ = configure.Restore(snap)
				}
			}()

			reports := lifecycle.Apply(ctx, desired, isContainer, jsonOutput)
			failed := false
			for _, r := range reports {
				r.Finish(jsonOutput)
				if !r.Success {
					failed = true
				}
			}

			if failed {
				_ = configure.Restore(snap)
				return fmt.Errorf("one or more steps failed during apply")
			}

			if err := config.Save(statePath, desired); err != nil {
				return err
			}

			if !jsonOutput {
				color.New(color.FgGreen, color.Bold).Println("✅ Apply complete.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", "config.toml", "Path to desired config")
	cmd.Flags().StringVar(&statePath, "state", "/var/lib/openldap-setup/state.toml", "Path to state file")
	cmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output diff and reports in JSON format")
	cmd.Flags().BoolVar(&isContainer, "container", false, "Install in a container environment")
	return cmd
}
