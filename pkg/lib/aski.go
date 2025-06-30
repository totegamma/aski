package lib

import (
	"fmt"
	"github.com/kznrluk/aski/pkg/config"
	"github.com/kznrluk/aski/pkg/conv"
	"github.com/kznrluk/aski/pkg/file"
	"github.com/spf13/cobra"
	"io"
	"log/slog"
	"os"
	"strings"
)

func Aski(cmd *cobra.Command, args []string) {
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

	var ctx conv.Conversation
	if restore != "" {
		load, fileName, err := ReadFileFromPWDAndHistoryDir(restore)
		if err != nil {
			slog.Error(fmt.Sprintf("error reading restore file: %v", err))
			os.Exit(1)
		}

		ctx, err = conv.FromYAML(load)
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
		ctx = conv.NewConversation(prof)
		ctx.SetSystem(prof.SystemContext)

		if len(fileGlobs) != 0 {
			fileContents := file.GetFileContents(fileGlobs)
			for _, f := range fileContents {
				if content == "" && !isPipe {
					slog.Info(fmt.Sprintf("Append File: %s", f.Name))
				}
				ctx.Append(conv.ChatRoleUser, fmt.Sprintf("Path: `%s`\n ```\n%s```", f.Path, f.Contents))
			}
		}

		for _, i := range prof.Messages {
			switch strings.ToLower(i.Role) {
			case conv.ChatRoleUser:
				ctx.Append(conv.ChatRoleUser, i.Content)
			case conv.ChatRoleAssistant:
				ctx.Append(conv.ChatRoleAssistant, i.Content)
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
		ctx.Append(conv.ChatRoleUser, string(s))
	}

	if content != "" {
		ctx.Append(conv.ChatRoleUser, content)
		_, err = OneShot(cfg, ctx, isRestMode)
		if err != nil {
			slog.Error(fmt.Sprintf("error in one-shot mode: %v", err))
			os.Exit(1)
		}
	} else {
		StartDialog(cfg, ctx, isRestMode, restore != "")
	}
}
