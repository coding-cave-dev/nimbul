package cli

import (
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Start watching Nimbul",
	Long:  `Initialize Nimbul`,
	RunE:  initExec,
}

func initExec(cmd *cobra.Command, args []string) error {
	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)
}
