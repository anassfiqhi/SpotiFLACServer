package backend

import (
	"context"
	"fmt"
)

func SelectMultipleFiles(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("not supported in server mode")
}

func SelectOutputDirectory(_ context.Context) (string, error) {
	return "", fmt.Errorf("not supported in server mode")
}
