package cmd

import (
	"fmt"
	"github.com/kznrluk/aski/pkg/config"
	"github.com/kznrluk/aski/pkg/conv"
	"github.com/kznrluk/aski/pkg/file"
	"github.com/kznrluk/aski/pkg/lib"
	"github.com/spf13/cobra"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var rootCmd = &cobra.Command{
	Use:   "aski",
	Args:  cobra.ArbitraryArgs,
	Short: "aski is a very small and user-friendly ChatGPT client.",
	Long:  `aski is a very small and user-friendly ChatGPT client. It works hard to maintain context and establish communication.`,
	Run:   aski,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		if verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})))
		} else {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})))
		}
	},
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
	rootCmd.Flags().StringP("model", "m", "", "Override the model to use for this conversation. This will override the model specified in the profile.")
	rootCmd.Flags().StringP("restore", "r", "", "Restore conversations from history yaml files. Search pwd and .aski/history folders by default. Prefix match.")
	rootCmd.Flags().BoolP("rest", "", false, "When you specify this flag, you will communicate with the REST API instead of streaming. This can be useful if the communication is unstable or if you are not receiving responses properly.")

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Debug logging")
}

func aski(cmd *cobra.Command, args []string) {
	profileTarget, err := cmd.Flags().GetString("profile")
	isRestMode, _ := cmd.Flags().GetBool("rest")
	model, _ := cmd.Flags().GetString("model")
	fileGlobs, _ := cmd.Flags().GetStringSlice("file")
	restore, _ := cmd.Flags().GetString("restore")
	content := strings.Join(args, " ")

	isPipe := false
	fileInfo, _ := os.Stdin.Stat()
	if (fileInfo.Mode() & os.ModeNamedPipe) != 0 {
		isPipe = true
	}

	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	if cfg.OpenAIAPIKey == "" && cfg.AnthropicAPIKey == "" {
		configPath := config.MustGetAskiDir()
		slog.Error(fmt.Sprintf("No API key found. Please set your API key in %s/config.yaml", configPath))
		os.Exit(1)
	}

	prof, err := config.GetProfile(cfg, profileTarget)
	if err != nil {
		slog.Error(fmt.Sprintf("error getting profile: %v. using default profile.", err))
		prof = config.InitialProfile()
	}

	if model != "" {
		prof.Model = model
	}

	var cv conv.Conversation
	if restore != "" {
		histPath := config.MustGetHistoryDir()
		if !strings.HasSuffix(restore, ".yaml") {
			restore += ".yaml"
		}
		path := filepath.Join(histPath, restore)
		fileName := filepath.Base(path)
		load, err := os.ReadFile(path)

		cv, err = conv.FromYAML(load, fileName)
		if err != nil {
			slog.Error(fmt.Sprintf("error parsing restore file: %v", err))
			os.Exit(1)
		}

		if len(fileGlobs) != 0 {
			// TODO: We should be able to renew file contents from the globs
			slog.Warn("File globs are ignored when loading restore.")
		}

		if profileTarget != "" {
			slog.Warn("Profile is ignored when loading restore.")
		}

		slog.Info(fmt.Sprintf("Restoring conversation from %s", fileName))
	} else {
		cv = conv.NewConversation(prof)
		cv.SetSystem(prof.SystemContext)

		if len(fileGlobs) != 0 {
			fileContents := file.GetFileContents(fileGlobs)
			for _, f := range fileContents {
				if content == "" && !isPipe {
					slog.Info(fmt.Sprintf("Append File: %s", f.Name))
				}
				cv.Append(conv.ChatRoleUser, fmt.Sprintf("Path: `%s`\n ```\n%s```", f.Path, f.Contents))
			}
		}

		for _, i := range prof.Messages {
			switch strings.ToLower(i.Role) {
			case conv.ChatRoleUser:
				cv.Append(conv.ChatRoleUser, i.Content)
			case conv.ChatRoleAssistant:
				cv.Append(conv.ChatRoleAssistant, i.Content)
			default:
				panic(fmt.Errorf("invalid role: %s", i.Role))
			}
		}
	}

	if isPipe {
		s, err := io.ReadAll(os.Stdin)
		if err != nil {
			slog.Error(fmt.Sprintf("error reading from stdin: %v", err))
			os.Exit(1)
		}
		cv.Append(conv.ChatRoleUser, string(s))
	}

	if content != "" {
		cv.Append(conv.ChatRoleUser, content)
		_, err = lib.OneShot(cfg, cv, isRestMode)
		if err != nil {
			slog.Error(fmt.Sprintf("error in one-shot mode: %v", err))
			os.Exit(1)
		}
	} else {
		lib.StartDialog(cfg, cv, isRestMode)
	}
}
