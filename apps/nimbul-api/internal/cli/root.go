/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nimbul",
	Short: "Nimbul is a self-hosted, Kubernetes-native platform for deploying apps on your own infrastructure.",
	Long:  `Nimbul is a self-hosted, Kubernetes-native platform for deploying apps on your own infrastructure.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

}
