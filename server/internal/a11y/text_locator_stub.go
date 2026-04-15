//go:build !darwin

package a11y

import (
	"context"
	"fmt"
)

func LocateTextInImage(_ context.Context, _ string) ([]TextLine, error) {
	return nil, fmt.Errorf("local text locator is only available on darwin")
}

func LocateTextInPNG(_ context.Context, _ []byte) ([]TextLine, error) {
	return nil, fmt.Errorf("local text locator is only available on darwin")
}
