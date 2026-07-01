//go:build !windows

package update

import (
	"os/exec"
)

func setSysProcAttr(_ *exec.Cmd) {
	// No-op on non-Windows platforms
}
