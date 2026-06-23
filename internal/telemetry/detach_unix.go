//go:build !windows

package telemetry

import "syscall"

// detachSysProcAttr starts the reporter in its own session (Setsid) so it is fully detached from
// this process and does not receive SIGHUP when we exit.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
