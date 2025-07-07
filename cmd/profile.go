package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/kznrluk/aski/pkg/config"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
	"strings"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Select profile.",
	Long: "Profiles are usually located in the .aski/config.yaml file in the home directory." +
		"By using profiles, you can easily switch between different conversation contexts on the fly.",
	Run: ChangeProfile,
}

func init() {
	rootCmd.AddCommand(profileCmd)
}

func ChangeProfile(cmd *cobra.Command, args []string) {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	profileDir := config.MustGetProfileDir()

	var yamlFiles []string

	fileInfo, err := os.ReadDir(profileDir)
	if err != nil {
		panic(err)
	}

	for _, file := range fileInfo {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") && file.Name() != "config.yaml" {
			yamlFiles = append(yamlFiles, file.Name())
		}
	}

	var selected string
	prompt := &survey.Select{
		Message: "Choose one option:",
		Options: yamlFiles,
	}

	_ = survey.AskOne(prompt, &selected)

	cfg.CurrentProfile = selected

	if err := config.Save(cfg); err != nil {
		slog.Error("Error saving config", "error", err)
		os.Exit(1)
	}
	return
}
