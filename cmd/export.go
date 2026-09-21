package cmd

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <shell>",
	Short: "Emit shell code to sync the process environment with nvy's stores",
	Args:  cobra.ExactArgs(1),
	RunE:  runExport,
}

func runExport(_ *cobra.Command, args []string) error {
	shell := args[0]

	if shell != "powershell" {
		fmt.Printf("# nvy: export for %s not yet implemented\n", shell)
		return nil
	}

	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	locals, err := store.LoadEnv(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	desired := make(map[string]string, len(gs)+len(locals))
	for k, entry := range gs {
		desired[k] = entry.Value
	}
	for k, v := range locals {
		desired[k] = v
	}

	prevApplied := splitApplied(os.Getenv("NVY_APPLIED"))

	fmt.Print(renderPowershellExport(desired, prevApplied))
	return nil
}

func splitApplied(watermark string) []string {
	if watermark == "" {
		return nil
	}
	parts := strings.Split(watermark, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			keys = append(keys, p)
		}
	}
	return keys
}

var safeEnvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// renderPowershellExport is pure codegen: it takes the desired environment and the
// previously-applied watermark and returns the PowerShell script that reconciles the
// live Process environment to match desired, unsetting anything that fell out.
func renderPowershellExport(desired map[string]string, prevApplied []string) string {
	var sb strings.Builder

	keys := make([]string, 0, len(desired))
	for k := range desired {
		if k == "NVY_APPLIED" || !safeEnvKey.MatchString(k) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	desiredSet := make(map[string]bool, len(keys))
	for _, k := range keys {
		desiredSet[k] = true
		sb.WriteString(fmt.Sprintf("[Environment]::SetEnvironmentVariable('%s','%s','Process')\n", k, psQuote(desired[k])))
	}

	prevKeys := make([]string, len(prevApplied))
	copy(prevKeys, prevApplied)
	sort.Strings(prevKeys)
	for _, k := range prevKeys {
		if k == "NVY_APPLIED" || !safeEnvKey.MatchString(k) {
			continue
		}
		if !desiredSet[k] {
			sb.WriteString(fmt.Sprintf("[Environment]::SetEnvironmentVariable('%s',$null,'Process')\n", k))
		}
	}

	sb.WriteString(fmt.Sprintf("$env:NVY_APPLIED='%s'\n", psQuote(strings.Join(keys, ","))))

	return sb.String()
}

func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
