package backend

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

func OpenFolderInExplorer(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func SelectFolderDialog(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("not supported in server mode")
}

func SelectFileDialog(_ context.Context) (string, error) {
	return "", fmt.Errorf("not supported in server mode")
}

func SelectImageVideoDialog(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("not supported in server mode")
}
