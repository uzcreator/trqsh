//go:build windows

package cli

import "syscall"

// Windows creation flags for a fully detached background process. DETACHED_PROCESS
// severs the child from the parent console (so it survives the terminal closing),
// and CREATE_NEW_PROCESS_GROUP keeps a Ctrl+C in the parent from propagating to it.
const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// detachSysProcAttr returns the attrs that launch `trqsh daemon` as an independent
// background process not tied to this CLI invocation's console.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: detachedProcess | createNewProcessGroup,
	}
}
