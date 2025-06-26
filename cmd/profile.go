package cmd

import (
	"github.com/kznrluk/aski/pkg/lib"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Select profile.",
	Long: "Profiles are usually located in the .aski/config.yaml file in the home directory." +
		"By using profiles, you can easily switch between different conversation contexts on the fly.",
	Run: lib.ChangeProfile,
}

func init() {
	rootCmd.AddCommand(profileCmd)
}
