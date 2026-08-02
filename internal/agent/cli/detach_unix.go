//go:build !windows

package cli

import "syscall"

// detachSysProcAttr starts `trqsh daemon` in a new session (setsid) so it is
// reparented away from this CLI process and survives the terminal closing.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
