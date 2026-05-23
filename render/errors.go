package render

import (
	"errors"
	"fmt"
	"strings"
)

var ErrSurfaceLost = errors.New("wgpu surface lost or outdated")

var recoverableSurfaceMessages = []string{
	"Surface timed out",
	"Surface is outdated",
	"Surface was lost",
	"Outdated",
}

func isSurfaceLost(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, substring := range recoverableSurfaceMessages {
		if strings.Contains(message, substring) {
			return true
		}
	}
	return false
}

func wrapSurfaceErr(err error) error {
	if isSurfaceLost(err) {
		return fmt.Errorf("%w: %v", ErrSurfaceLost, err)
	}
	return err
}
