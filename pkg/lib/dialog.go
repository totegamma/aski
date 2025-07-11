package lib

import (
	"context"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/kznrluk/aski/pkg/chat"
	"github.com/kznrluk/aski/pkg/command"
	"github.com/kznrluk/aski/pkg/config"
	"github.com/kznrluk/aski/pkg/conv"
	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/coloring"
	"github.com/nyaosorg/go-readline-ny/simplehistory"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func StartDialog(cfg config.Config, cv conv.Conversation, isRestMode bool) {

	if isRestMode {
		fmt.Printf("REST Mode \n")
	}

	history := simplehistory.New()

	profile := cv.GetProfile()
	editor := &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			return io.WriteString(w, "\u001B[0m"+profile.UserName+"@"+profile.ProfileName+"> ") // print `$ ` with cyan
		},
		Writer:         colorable.NewColorableStdout(),
		History:        history,
		Coloring:       &coloring.VimBatch{},
		HistoryCycling: true,
	}

	editor.Init()
	fmt.Printf("Profile: %s, Model: %s \n", profile.ProfileName, profile.Model)

	cli, err := chat.ProvideChat(profile.Vendor, profile.Model, cfg)
	if err != nil {
		fmt.Printf("error providing chat client: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if profile.AutoSave {
			fn, err := saveConversation(cv)
			if err != nil {
				fmt.Printf("\n error saving conversation: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(fn)
		}
	}()

	for {
		fmt.Printf("\n")
		editor.PromptWriter = func(w io.Writer) (int, error) {
			return io.WriteString(w, fmt.Sprintf("%.*s > ", 6, cv.Last().Sha1))
		}

		input, err := getInput(editor)
		if err != nil {
			if errors.Is(err, readline.CtrlC) {
				fmt.Println("\nSIGINT received, exiting...")
				return
			} else if errors.Is(err, io.EOF) {
				return
			}
			fmt.Printf("Error Occured: %v\n", err)
			continue
		}

		history.Add(input)

		if input == "" {
			continue
		}

		if input[0] == ':' {
			newcv, cont, commandErr := command.Parse(input, cv)
			if commandErr != nil {
				if errors.Is(commandErr, command.ErrShouldExit) {
					return
				}
				fmt.Printf("error: %v\n", commandErr)
			}

			if !cont {
				continue
			}

			cv = newcv
		} else {
			cv.Append(conv.ChatRoleUser, input)
		}

		last := cv.Last()
		yellow := color.New(color.FgHiYellow).SprintFunc()
		fmt.Print(yellow(fmt.Sprintf("\n%s -> [%.*s] \n", last.Role, 6, last.ParentSha1)))
		fmt.Print(fmt.Sprintf("%s", last.Content))
		fmt.Print(yellow(fmt.Sprintf(" [%.*s]\n", 6, last.Sha1)))

		messages := cv.MessagesFromHead()
		if len(messages) > 0 {
			lastMessage := messages[len(messages)-1]
			showPendingHeader(conv.ChatRoleAssistant, lastMessage)
		}

		fmt.Printf("\n")
		data, err := cli.Retrieve(cv, isRestMode)
		if err != nil {
			if errors.Is(err, chat.ErrCancelled) {
				_, _ = cv.ChangeHead(last.ParentSha1)
				continue
			}
			fmt.Printf("\n%s", err.Error())
			continue
		}

		msg := cv.Append(conv.ChatRoleAssistant, data)
		fmt.Print(yellow(fmt.Sprintf(" [%.*s]\n", 6, msg.Sha1)))
	}
}

func OneShot(cfg config.Config, cv conv.Conversation, isRestMode bool) (string, error) {
	defer func() {
		if cv.GetProfile().AutoSave {
			fn, err := saveConversation(cv)
			if err != nil {
				fmt.Printf("\n error saving conversation: %v\n", err)
			} else {
				fmt.Println(fn)
			}
		}
	}()

	profile := cv.GetProfile()
	cli, err := chat.ProvideChat(profile.Vendor, profile.Model, cfg)
	if err != nil {
		return "", fmt.Errorf("error providing chat client: %v", err)
	}

	data, err := cli.Retrieve(cv, isRestMode)

	fmt.Printf("\n") // in some cases, shell prompt delete the last line so we add a new line
	if err != nil {
		fmt.Println(err.Error())
		return "", nil
	}

	cv.Append(conv.ChatRoleAssistant, data)

	return data, nil
}

func getInput(reader *readline.Editor) (string, error) {

	ctx := context.Background()

	input, err := reader.ReadLine(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func saveConversation(conv conv.Conversation) (string, error) {
	t := time.Now()

	if len(conv.GetMessages()) == 0 {
		return "", nil
	}

	filename := conv.GetFilename()
	if filename == "" {
		filename = fmt.Sprintf("%s.yaml", t.Format("20060102-150405"))
	}

	homeDir, err := config.GetHomeDir()
	if err != nil {
		return "", fmt.Errorf("error getting home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".aski", "history")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return filename, err
	}

	yamlString, err := conv.ToYAML()
	if err != nil {
		return filename, err
	}

	filePath := filepath.Join(configDir, filename)
	err = os.WriteFile(filePath, yamlString, 0600)
	if err != nil {
		return filename, err
	}

	return filename, nil
}

func showPendingHeader(role string, to conv.Message) {
	yellow := color.New(color.FgHiYellow).SprintFunc()
	fmt.Print(yellow(fmt.Sprintf("\n%s -> [%.*s]", role, 6, to.Sha1)))
}
