package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/AgusRdz/nvy/internal/notify"
	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for expiring variables and send notifications",
	Args:  cobra.NoArgs,
	RunE:  runCheck,
}

func runCheck(_ *cobra.Command, _ []string) error {
	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	var fired int

	// global vars
	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	for key, entry := range gs {
		if entry.ExpiresAt == nil {
			continue
		}
		days := daysUntil(*entry.ExpiresAt)
		if days <= cfg.NotificationLeadDays {
			if err := notify.Expiring(key, days); err != nil {
				fmt.Fprintf(os.Stderr, "nvy: notify %s: %v\n", key, err)
			}
			fired++
		}
	}

	// local vars (current directory, best-effort)
	dir, err := os.Getwd()
	if err == nil {
		meta, err := store.LoadLocalMeta(dir)
		if err == nil {
			for key, entry := range meta {
				if entry.ExpiresAt == nil {
					continue
				}
				days := daysUntil(*entry.ExpiresAt)
				if days <= cfg.NotificationLeadDays {
					if err := notify.Expiring(key, days); err != nil {
						fmt.Fprintf(os.Stderr, "nvy: notify %s: %v\n", key, err)
					}
					fired++
				}
			}
		}
	}

	if fired == 0 {
		fmt.Println("nvy: no expiring variables")
	}
	return nil
}

func daysUntil(t time.Time) int {
	return int(time.Until(t).Hours() / 24)
}
