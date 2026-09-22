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
	if !cfg.NotificationsOn() {
		fmt.Println("nvy: notifications disabled")
		return nil
	}

	state, err := store.LoadNotifyState()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	today := time.Now().Format("2006-01-02")

	var due int

	notifyOnce := func(key string, days int) {
		due++
		if !shouldNotify(state[key], today) {
			return
		}
		if err := notify.Expiring(key, days); err != nil {
			fmt.Fprintf(os.Stderr, "nvy: notify %s: %v\n", key, err)
		}
		state[key] = today
	}

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
			notifyOnce(key, days)
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
					notifyOnce(key, days)
				}
			}
		}
	}

	if err := store.SaveNotifyState(state); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if due == 0 {
		fmt.Println("nvy: no expiring variables")
	}
	return nil
}

func daysUntil(t time.Time) int {
	return int(time.Until(t).Hours() / 24)
}

// shouldNotify reports whether a var last notified on lastNotified should be
// notified again on today. True unless it was already notified today.
func shouldNotify(lastNotified, today string) bool {
	return lastNotified != today
}
