package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/importer"
	"github.com/kianurivzzz/save-serve-cli/internal/sshcfg"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import hosts from other sources",
}

var importSSHConfigCmd = &cobra.Command{
	Use:   "ssh-config [path]",
	Short: "Import Host entries from ~/.ssh/config into group \"imported\"",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runImportSSHConfig,
}

func init() {
	importSSHConfigCmd.Flags().Bool("overwrite", false, "replace hosts that already exist")
	importCmd.AddCommand(importSSHConfigCmd)
	rootCmd.AddCommand(importCmd)
}

func runImportSSHConfig(cmd *cobra.Command, args []string) error {
	path := sshcfg.Path()
	if len(args) == 1 {
		path = config.ExpandHome(args[0])
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hosts, err := importer.ParseSSHConfig(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotFound) {
		cfg, err = config.New(), nil
	}
	if err != nil {
		return err
	}
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	res := importer.Import(cfg, hosts, overwrite)
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintf(os.Stderr, "skipped %d existing: %s (use --overwrite to replace)\n", len(res.Skipped), strings.Join(res.Skipped, ", "))
	}
	if len(res.Added) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to import")
		return nil
	}
	if err := saveAndSync(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "imported %d hosts into group %q: %s\n", len(res.Added), importer.Group, strings.Join(res.Added, ", "))
	return nil
}
