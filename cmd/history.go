package cmd

import (
	"fmt"
	"github.com/kznrluk/aski/pkg/conv"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"sort"
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

	sort.Slice(files, func(i, j int) bool {
		fi, _ := files[i].Info()
		fj, _ := files[j].Info()
		return fi.ModTime().After(fj.ModTime())
	})

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(historyDir, file.Name()))
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", file.Name(), err)
			continue
		}

		ctx, err := conv.FromYAML(bytes, file.Name())
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
		content := strings.ReplaceAll(root.Content, "\n", " ")
		if len(content) > 50 {
			content = content[:50] + "..."
		}

		fmt.Printf("%s %s\n", name, content)
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

	baseName := filepath.Base(filePath)
	ctx, err := conv.FromYAML(bytes, baseName)
	if err != nil {
		fmt.Printf("Error parsing file %s: %v\n", filePath, err)
		return
	}

	ctx.Print()

}

func init() {
	rootCmd.AddCommand(historyCmd)
}
