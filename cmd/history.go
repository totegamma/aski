package cmd

import (
	"fmt"
	"github.com/kznrluk/aski/pkg/conv"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Select history.",
	Long: "historys are usually located in the .aski/config.yaml file in the home directory." +
		"By using historys, you can easily switch between different conversation contexts on the fly.",
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			list()
		} else {
			single(args)
		}
	},
}

func list() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	historyDir := filepath.Join(homeDir, ".aski", "history")
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		return
	}
	files, err := os.ReadDir(historyDir)
	if err != nil {
		fmt.Println("Error reading history directory:", err)
		return
	}
	if len(files) == 0 {
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(historyDir, file.Name()))
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", file.Name(), err)
			continue
		}

		ctx, err := conv.FromYAML(bytes)
		if err != nil {
			fmt.Printf("Error parsing file %s: %v\n", file.Name(), err)
			continue
		}

		root, err := ctx.GetRootMessage()
		if err != nil {
			fmt.Printf("Error getting root message from file %s: %v\n", file.Name(), err)
			continue
		}

		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

		fmt.Printf("%s %s\n", name, root.Content)
	}
}

func single(args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	historyDir := filepath.Join(homeDir, ".aski", "history")
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		fmt.Println("History directory does not exist.")
		return
	}

	filePath := filepath.Join(historyDir, args[0]+".yaml")
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", filePath, err)
		return
	}

	ctx, err := conv.FromYAML(bytes)
	if err != nil {
		fmt.Printf("Error parsing file %s: %v\n", filePath, err)
		return
	}

	for _, msg := range ctx.GetMessages() {
		fmt.Printf("%s: %s\n", msg.Role, msg.Content)
	}
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
