package command

import (
	"context"
	"time"

	"github.com/kitechsoftware/ldappy/internal/cli/status"
	"github.com/kitechsoftware/ldappy/internal/common/ui/log"
	"github.com/spf13/cobra"
)

// StatusCmd displays OpenLDAP service and configuration status.
func StatusCmd(ctx context.Context) *cobra.Command {
	var watchInterval int

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show OpenLDAP service and configuration status",
		Long: `Display the current state of OpenLDAP (slapd), configuration, and connectivity.
Use --watch N to refresh the status every N seconds until interrupted.`,
		RunE: func(c *cobra.Command, args []string) error {
			jsonOutput, _ := c.Root().PersistentFlags().GetBool("json")
			log.SetJSONOutput(jsonOutput)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if !jsonOutput {
				log.Section("OpenLDAP Status")
			}

			// single-shot mode
			if watchInterval <= 0 {
				r, err := status.Collect(ctx)
				if err != nil {
					return err
				}
				if jsonOutput {
					log.JSON(r)
				} else {
					renderStatus(r)
					log.SectionEnd()
				}
				return nil
			}

			// watch mode
			ticker := time.NewTicker(time.Duration(watchInterval) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					log.WarnCtx(ctx, "Status watch stopped: %v", ctx.Err())
					if !jsonOutput {
						log.SectionEnd()
					}
					return ctx.Err()
				case <-ticker.C:
					r, err := status.Collect(ctx)
					if err != nil {
						log.ErrorCtx(ctx, "collect failed: %v", err)
						continue
					}
					if jsonOutput {
						log.JSON(r)
					} else {
						renderStatus(r)
					}
				}
			}
		},
	}

	cmd.Flags().IntVarP(&watchInterval, "watch", "w", 0, "Refresh status every N seconds")
	return cmd
}

func renderStatus(r status.StatusReport) {
	log.Subsection("Service")
	if r.Service.Active {
		log.Success("slapd active since %s", r.Service.Since)
	} else {
		log.Warn("slapd inactive")
	}

	log.Subsection("Ports")
	if r.Ports.LDAP {
		log.Success("LDAP :389 open")
	} else {
		log.Warn("LDAP :389 closed")
	}
	if r.Ports.LDAPS {
		log.Success("LDAPS :636 open")
	} else {
		log.Warn("LDAPS :636 closed")
	}

	if r.BaseDN != "" {
		log.Info("BaseDN: %s", r.BaseDN)
	}
	if r.TLS.Exists {
		log.Info("TLS cert expires %s (%dd)", r.TLS.ExpiryDate, r.TLS.ExpiresInDays)
	} else {
		log.Warn("TLS cert: not found")
	}
	if r.LastBackup != "" {
		log.Info("Last backup: %s", r.LastBackup)
	}
	log.Info("Config version: %s", r.ConfigVersion)
}
