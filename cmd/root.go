package cmd

import (
	"github.com/kznrluk/aski/lib"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "aski",
	Short: "aski is a very small and user-friendly ChatGPT client.",
	Long:  `aski is a very small and user-friendly ChatGPT client. It works hard to maintain context and establish communication.`,
	Run:   lib.Aski,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringSliceP("file", "f", []string{}, "Input file(s) to start dialog from. Can be specified multiple times.")
	rootCmd.Flags().StringP("profile", "p", "", "Select the profile to use for this conversation, as defined in the .aski/config.yaml file.")
	rootCmd.Flags().StringP("content", "c", "", "Input text to start dialog from command line")
	rootCmd.Flags().StringP("model", "m", "", "Override the model to use for this conversation. This will override the model specified in the profile.")
	rootCmd.Flags().StringP("restore", "r", "", "Restore conversations from history yaml files. Search pwd and .aski/history folders by default. Prefix match.")
	rootCmd.Flags().BoolP("rest", "", false, "When you specify this flag, you will communicate with the REST API instead of streaming. This can be useful if the communication is unstable or if you are not receiving responses properly.")
	rootCmd.Flags().BoolP("verbose", "v", false, "Debug logging")

}
