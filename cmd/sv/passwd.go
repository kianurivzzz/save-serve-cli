package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

var passwdCmd = &cobra.Command{
	Use:               "passwd <name>",
	Short:             "Store or update the password for a host and switch it to auth: password",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeHostFirstArg,
	RunE:              runPasswd,
}

func init() {
	passwdCmd.Flags().String("store", "", "where to keep the password: keychain or file")
	passwdCmd.RegisterFlagCompletionFunc("store", completeStore)
	rootCmd.AddCommand(passwdCmd)
}

func runPasswd(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	h := cfg.Find(args[0])
	if h == nil {
		return fmt.Errorf("host %q not found", args[0])
	}
	storeFlag, _ := cmd.Flags().GetString("store")
	store, err := openStore(cfg, storeFlag)
	if err != nil {
		return err
	}
	pw, err := promptPassword(h.Name)
	if err != nil {
		return err
	}
	if err := store.Set(h.Name, pw); err != nil {
		return err
	}
	if h.Auth != config.AuthPassword {
		h.Auth = config.AuthPassword
		if err := saveAndSync(cfg); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "password for %s saved to %s\n", h.Name, store.Kind())
	return nil
}

func promptPassword(name string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("no password on stdin")
		}
		pw := strings.TrimRight(line, "\r\n")
		if pw == "" {
			return "", errors.New("password is empty")
		}
		return pw, nil
	}
	first, err := readHidden(fmt.Sprintf("password for %s: ", name))
	if err != nil {
		return "", err
	}
	if first == "" {
		return "", errors.New("password is empty")
	}
	second, err := readHidden("repeat: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passwords do not match")
	}
	return first, nil
}

func readHidden(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func completeStore(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"keychain", "file"}, cobra.ShellCompDirectiveNoFileComp
}
