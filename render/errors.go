package render

import (
	"errors"
	"fmt"
	"strings"
)

var ErrSurfaceLost = errors.New("wgpu surface lost or outdated")

var (
	// ErrGraphNotCompiled is returned by Execute when Compile has not run.
	ErrGraphNotCompiled = errors.New("render: graph not compiled")
	// ErrGraphCycle is returned by Compile when the passes form a dependency cycle.
	ErrGraphCycle = errors.New("render: cycle in render graph dependencies")
	// ErrSlotNotBound reports a pass slot with no bound resource.
	ErrSlotNotBound = errors.New("render: pass slot not bound")
	// ErrSlotNoView reports a bound slot whose resource has no texture view.
	ErrSlotNoView = errors.New("render: pass slot has no view")
)

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
