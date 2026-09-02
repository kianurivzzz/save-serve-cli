package main

import (
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

var importTermiusCmd = &cobra.Command{
	Use:   "termius <file.csv>",
	Short: "Import a Termius CSV export, passwords go to the keychain",
	Long: `Import hosts from a CSV file exported by Termius.

Expected columns: Groups, Label, Tags, Hostname/IP, Protocol, Port, Username, Password.
Groups are kept as they are, hosts without a group land in "imported".
Rows with a password become auth: password and the password is stored in the keychain.
Rows with a protocol other than ssh are skipped.`,
	Args: cobra.ExactArgs(1),
	RunE: runImportTermius,
}

func init() {
	importSSHConfigCmd.Flags().Bool("overwrite", false, "replace hosts that already exist")
	importTermiusCmd.Flags().Bool("overwrite", false, "replace hosts that already exist")
	importTermiusCmd.Flags().String("store", "", "where to keep passwords: keychain or file")
	importTermiusCmd.RegisterFlagCompletionFunc("store", completeStore)
	importCmd.AddCommand(importSSHConfigCmd, importTermiusCmd)
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
	cfg, err := loadOrNewConfig()
	if err != nil {
		return err
	}
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	res := importer.Import(cfg, hosts, overwrite)
	if !reportImport(res, nil) {
		return nil
	}
	if err := saveAndSync(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "imported %d hosts into group %q: %s\n", len(res.Added), importer.Group, strings.Join(res.Added, ", "))
	return nil
}

func runImportTermius(cmd *cobra.Command, args []string) error {
	path := config.ExpandHome(args[0])
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	parsed, warnings, err := importer.ParseTermius(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	cfg, err := loadOrNewConfig()
	if err != nil {
		return err
	}
	hosts := make([]config.Host, 0, len(parsed))
	passwords := make(map[string]string)
	for _, p := range parsed {
		if p.Password != "" {
			passwords[strings.ToLower(p.Host.Name)] = p.Password
		} else {
			p.Host.Auth = cfg.DefaultAuth()
		}
		hosts = append(hosts, p.Host)
	}
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	res := importer.Import(cfg, hosts, overwrite)
	if !reportImport(res, warnings) {
		return nil
	}
	stored := 0
	if len(passwords) > 0 {
		storeFlag, _ := cmd.Flags().GetString("store")
		store, err := openStore(cfg, storeFlag)
		if err != nil {
			return err
		}
		for _, name := range res.Added {
			pw, ok := passwords[strings.ToLower(name)]
			if !ok {
				continue
			}
			if err := store.Set(name, pw); err != nil {
				return fmt.Errorf("store password for %s: %w", name, err)
			}
			stored++
		}
		if stored > 0 {
			fmt.Fprintf(os.Stderr, "stored %d passwords in %s\n", stored, store.Kind())
		}
	}
	if err := saveAndSync(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "imported %d hosts: %s\n", len(res.Added), strings.Join(res.Added, ", "))
	return nil
}

func reportImport(res importer.Result, warnings []string) bool {
	for _, w := range append(warnings, res.Warnings...) {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintf(os.Stderr, "skipped %d existing: %s (use --overwrite to replace)\n", len(res.Skipped), strings.Join(res.Skipped, ", "))
	}
	if len(res.Added) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to import")
		return false
	}
	return true
}
