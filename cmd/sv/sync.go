package main

import "github.com/spf13/cobra"

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Rebuild the sv block in ~/.ssh/config from hosts.yaml",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return syncSSHConfig(cfg)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
