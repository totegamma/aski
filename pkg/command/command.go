package command

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/kznrluk/aski/pkg/config"
	"github.com/kznrluk/aski/pkg/conv"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

var (
	ErrShouldExit = errors.New("should exit")
)

type cmdFn func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error)

type cmd struct {
	name        string
	aliases     []string
	description string
	exec        cmdFn
}

var availableCommands = []cmd{
	{
		name:        ":history",
		description: "Show conversation history.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			conv.Print()
			return nil, false, nil
		},
	},
	{
		name:        ":move",
		description: "Change HEAD to another message.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			if len(commands) < 2 {
				return nil, false, fmt.Errorf("no SHA1 partial provided")
			}
			err := changeHead(commands[1], conv)
			return nil, false, err
		},
	},
	{
		name:        ":config",
		description: "Open configuration directory.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			_ = config.OpenConfigDir()
			return nil, false, nil
		},
	},
	{
		name: ":editor",
		description: "Open an external text editor to add new message.\n" +
			"  :editor sha1   - Edit the argument message and continue the conversation.\n" +
			"  :editor latest - Edits the nearest own statement from HEAD.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			trim := ""
			if len(commands) > 1 {
				trim = strings.TrimSpace(commands[1])
			}

			if trim == "" {
				return newMessage(conv)
			}

			return editMessage(conv, trim)
		},
	},
	{
		name: ":modify sha1",
		description: "Modify the past conversation. HEAD does not move.\n" +
			"                   Past conversations will be modified from the next transmission.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			if len(commands) < 2 {
				return nil, false, fmt.Errorf("no SHA1 provided")
			}
			return modifyMessage(conv, commands[1])
		},
	},
	{
		name: ":param",
		description: "Update profile custom parameter values.\n" +
			"                   There is no need to change it for normal use.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			if len(commands) < 3 {
				if len(commands) == 2 {
					displayParameterValue(conv.GetProfile().CustomParameters, commands[1])
					return conv, false, nil
				}
				fmt.Printf(customParametersDescription())
				return nil, false, nil
			}

			cv, err := setProfileCustomParamValue(conv, commands[1], commands[2])
			if err != nil {
				return nil, false, err
			}
			return cv, false, err
		},
	},
	{
		name:        ":exit",
		aliases:     []string{":q", ":quit"},
		description: "Exit the program.",
		exec: func(commands []string, conv conv.Conversation) (conv.Conversation, bool, error) {
			return nil, false, ErrShouldExit
		},
	},
}

func matchCommand(input string) (*cmd, bool) {

	var matchedCmd *cmd
	overlap := 0
	collision := false

	for _, cmd := range availableCommands {
		if cmd.name == input {
			return &cmd, true // exact match
		}

		if len(input) < len(cmd.name) && strings.HasPrefix(cmd.name, input) {
			if len(input) == overlap {
				collision = true
			} else {
				overlap = len(input)
				matchedCmd = &cmd
			}
		}

		for _, alias := range cmd.aliases {
			if alias == input {
				return &cmd, true
			}
		}
	}

	return matchedCmd, matchedCmd != nil && !collision
}

func unknownCommand() string {
	output := "unknown command.\n\n"
	for _, cmd := range availableCommands {
		output += fmt.Sprintf("  %-14s - %s\n", cmd.name, cmd.description)
	}

	return output
}

func Parse(input string, conv conv.Conversation) (conv.Conversation, bool, error) {
	trimmedInput := strings.TrimSpace(input)
	commands := strings.Split(trimmedInput, " ")

	matchedCmd, found := matchCommand(commands[0])
	if !found {
		return nil, false, fmt.Errorf(unknownCommand())
	}

	return matchedCmd.exec(commands, conv)
}

func changeHead(sha1Partial string, context conv.Conversation) error {
	if sha1Partial == "" {
		return fmt.Errorf("No SHA1 partial provided")
	}
	msg, err := context.ChangeHead(sha1Partial)
	if err != nil {
		return err
	}

	yellow := color.New(color.FgHiYellow).SprintFunc()
	blue := color.New(color.FgHiBlue).SprintFunc()
	fmt.Printf("%s %s\n", yellow(yellow(fmt.Sprintf("%.*s [%s] -> %.*s", 6, msg.Sha1, msg.Role, 6, msg.ParentSha1))), blue("Head"))
	for _, context := range strings.Split(msg.Content, "\n") {
		fmt.Printf("  %s\n", context)
	}

	return nil
}

func newMessage(cv conv.Conversation) (conv.Conversation, bool, error) {
	comments := "\n\n# Save and close editor to continue\n"
	s := cv.MessagesFromHead()
	for i := len(s) - 1; i >= 0; i-- {
		msg := s[i]
		head := ""
		if msg.Head {
			head = "Head"
		}

		d := fmt.Sprintf("#\n# %.*s -> %.*s [%s] %s\n", 6, msg.Sha1, 6, msg.ParentSha1, msg.Role, head)
		for _, context := range strings.Split(msg.Content, "\n") {
			d += fmt.Sprintf("#   %s\n", context)
		}
		comments += d
	}

	result, err := openEditor(comments)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open editor: %v", err)
	}

	if result == "" {
		return cv, false, nil
	}

	cv.Append(conv.ChatRoleUser, result)
	return cv, true, nil
}

func modifyMessage(cv conv.Conversation, sha1 string) (conv.Conversation, bool, error) {
	trimmedSha1 := strings.TrimSpace(sha1)
	if trimmedSha1 == "" {
		return nil, false, fmt.Errorf("no SHA1 provided")
	}

	msg, err := cv.GetMessageFromSha1(trimmedSha1)
	if err != nil {
		return nil, false, fmt.Errorf("failed to edit message from SHA1: %v", err)
	}

	s := cv.MessagesFromHead()
	comments := msg.Content + "\n\n# Save and close editor to continue\n"
	for i := len(s) - 1; i >= 0; i-- {
		m := s[i]
		head := ""
		if m.Head {
			head = "Head"
		}

		d := fmt.Sprintf("#\n# %.*s -> %.*s [%s] %s\n", 6, m.Sha1, 6, m.ParentSha1, m.Role, head)
		for _, context := range strings.Split(m.Content, "\n") {
			d += fmt.Sprintf("#   %s\n", context)
		}
		comments += d
	}

	result, err := openEditor(comments)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open editor: %v", err)
	}

	if result == "" {
		return cv, false, nil
	}

	if strings.TrimSpace(result) == strings.TrimSpace(msg.Content) {
		return cv, false, nil
	}

	msg.Content = result

	err = cv.Modify(msg)
	if err != nil {
		return nil, false, fmt.Errorf("failed to modify message: %v", err)
	}

	fmt.Printf("[%.6s] Modified. \n", msg.Sha1)
	return cv, false, nil
}

func editMessage(cv conv.Conversation, sha1 string) (conv.Conversation, bool, error) {
	trimmedSha1 := strings.TrimSpace(sha1)
	if trimmedSha1 == "" {
		return nil, false, fmt.Errorf("no SHA1 provided")
	}

	var msg conv.Message
	if strings.ToLower(trimmedSha1) == "latest" {
		msgs := cv.MessagesFromHead()
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == conv.ChatRoleUser {
				msg = msgs[i]
				break
			}
		}

		if msg.Sha1 == "" {
			return nil, false, fmt.Errorf("no latest user message found")
		}
	} else {
		m, err := cv.GetMessageFromSha1(trimmedSha1)
		if err != nil {
			return nil, false, fmt.Errorf("failed to edit message from SHA1: %v", err)
		}

		if m.Role != conv.ChatRoleUser {
			return nil, false, fmt.Errorf("cannot edit non-user message")
		}

		msg = m
	}

	s := cv.MessagesFromHead()
	comments := msg.Content + "\n\n# Save and close editor to continue\n"
	for i := len(s) - 1; i >= 0; i-- {
		m := s[i]
		head := ""
		if m.Head {
			head = "Head"
		}

		d := fmt.Sprintf("#\n# %.*s -> %.*s [%s] %s\n", 6, m.Sha1, 6, m.ParentSha1, m.Role, head)
		for _, context := range strings.Split(m.Content, "\n") {
			d += fmt.Sprintf("#   %s\n", context)
		}
		comments += d
	}

	result, err := openEditor(comments)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open editor: %v", err)
	}

	if result == "" {
		return cv, false, nil
	}

	if strings.TrimSpace(result) == strings.TrimSpace(msg.Content) {
		return cv, false, nil
	}

	_, err = cv.ChangeHead(msg.ParentSha1)
	if err != nil {
		return nil, false, fmt.Errorf("failed to change head: %v", err)
	}

	cv.Append(msg.Role, result)
	return cv, true, nil
}

func openEditor(content string) (string, error) {
	tempDir := config.MustGetAskiDir()
	tmpFile, err := os.CreateTemp(tempDir, "aski-editor-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create a temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	if err != nil {
		return "", fmt.Errorf("failed to write to the temp file: %v", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad.exe"
		} else {
			editor = "vim"
		}
	}

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// for vscode :)
	if strings.Contains(editor, "code") {
		cmd = exec.Command(editor, "--wait", tmpFile.Name())
	}

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to open editor: %v", err)
	}

	tmpFile.Close()

	rawContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read the edited content: %v", err)
	}

	result := ""
	for _, d := range strings.Split(string(rawContent), "\n") {
		if !strings.HasPrefix(d, "#") {
			result += d + "\n"
		}
	}
	result = strings.TrimSpace(result)
	if len(strings.TrimSpace(result)) == 0 {
		return "", nil
	}

	return result, nil
}

func setProfileCustomParamValue(conv conv.Conversation, paramName, paramValue string) (conv.Conversation, error) {
	targetProfile := conv.GetProfile()

	matchedParam := ""
	matched := false
	for _, param := range []string{"temperature", "stop", "logit_bias", "max_tokens", "top_p", "presence_penalty", "frequency_penalty"} {
		if strings.HasPrefix(param, paramName) {
			if matched {
				return nil, fmt.Errorf("ambiguous parameter name: %s", paramName)
			}
			matched = true
			matchedParam = param
		}
	}

	if !matched {
		return nil, fmt.Errorf("unknown custom parameter: %s", paramName)
	}

	switch matchedParam {
	case "temperature":
		newValue, err := strconv.ParseFloat(paramValue, 32)
		if err != nil {
			return nil, err
		}
		targetProfile.CustomParameters.Temperature = float32(newValue)
	case "stop":
		if len(paramValue) == 0 {
			targetProfile.CustomParameters.Stop = []string{}
		} else {
			stopValues := strings.Split(paramValue, ",")
			if len(stopValues) > 4 {
				return nil, fmt.Errorf("too many stop values provided, maximum 4 allowed")
			}
			targetProfile.CustomParameters.Stop = stopValues
		}
	case "logit_bias":
		return nil, fmt.Errorf("logit_bias can only be set via the profile")
	case "max_tokens":
		newValue, err := strconv.Atoi(paramValue)
		if err != nil {
			return nil, err
		}
		targetProfile.CustomParameters.MaxTokens = newValue
	case "top_p":
		newValue, err := strconv.ParseFloat(paramValue, 32)
		if err != nil {
			return nil, err
		}
		targetProfile.CustomParameters.TopP = float32(newValue)
	// case "n":
	// newValue, err := strconv.Atoi(paramValue)
	// if err != nil {
	// 	return nil, err
	// }
	// targetProfile.CustomParameters.N = newValue
	case "presence_penalty":
		newValue, err := strconv.ParseFloat(paramValue, 32)
		if err != nil {
			return nil, err
		}
		targetProfile.CustomParameters.PresencePenalty = float32(newValue)
	case "frequency_penalty":
		newValue, err := strconv.ParseFloat(paramValue, 32)
		if err != nil {
			return nil, err
		}
		targetProfile.CustomParameters.FrequencyPenalty = float32(newValue)
	default:
		return nil, fmt.Errorf("unknown custom parameter: %s", paramName)
	}

	// Validation.
	err := config.ValidateCustomParameters(targetProfile.CustomParameters)
	if err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}

	conv.SetProfile(targetProfile)
	displayParameterValue(conv.GetProfile().CustomParameters, matchedParam)
	return conv, nil
}

func customParametersDescription() string {
	return `Usage: :param <parameter_name> <parameter_value>

Available parameters:
  temperature       - What sampling temperature to use (0 to 2)
  top_p             - Nucleus sampling (tokens with top_p probability mass)
  stop              - Up to 4 sequences where the API will stop (comma-separated)
  max_tokens        - Maximum number of tokens to generate
  presence_penalty  - Penalize new tokens based on existing text
  frequency_penalty - Penalize new tokens based on frequency in text

If parameter_value is not provided, the current parameter value will be displayed. Use 0 to default.
`
}
func displayParameterValue(cp config.CustomParameters, paramName string) {
	matchedParam := ""
	matched := false
	for _, param := range []string{"temperature", "stop", "logit_bias", "max_tokens", "top_p", "presence_penalty", "frequency_penalty"} {
		if strings.HasPrefix(param, paramName) {
			if matched {
				fmt.Printf("ambiguous parameter name: %s\n", paramName)
				return
			}
			matched = true
			matchedParam = param
		}
	}
	if !matched {
		fmt.Printf("unknown custom parameter: %s", paramName)
		return
	}

	switch matchedParam {
	case "temperature":
		if cp.Temperature == 0.0 {
			fmt.Printf("Current temperature value: API Default\n")
			return
		}
		fmt.Printf("Current temperature value: %.2f\n", cp.Temperature)
	case "top_p":
		if cp.TopP == 0.0 {
			fmt.Printf("Current top_p value: API Default\n")
			return
		}
		fmt.Printf("Current top_p value: %.2f\n", cp.TopP)
	//case "n":
	//	fmt.Printf("Current value: %d\n", cp.N)
	case "stop":
		if len(cp.Stop) == 0 {
			fmt.Printf("Current stop values: API Default\n")
			return
		}
		fmt.Printf("Current stop values: %v\n", cp.Stop)
	case "max_tokens":
		if cp.MaxTokens == 0 {
			fmt.Printf("Current max_tokens value: API Default\n")
		}
		fmt.Printf("Current max_tokens value: %d\n", cp.MaxTokens)
	case "presence_penalty":
		if cp.PresencePenalty == 0 {
			fmt.Printf("Current presence_penalty value: API Default\n")
			return
		}
		fmt.Printf("Current presence_penalty value: %.2f\n", cp.PresencePenalty)
	case "frequency_penalty":
		if cp.FrequencyPenalty == 0 {
			fmt.Printf("Current frequency_penalty value: API Default\n")
			return
		}
		fmt.Printf("Current frequency_penalty value: %.2f\n", cp.FrequencyPenalty)
	default:
		fmt.Printf("Unknown parameter: %s\n%s", paramName, customParametersDescription())
	}
}
