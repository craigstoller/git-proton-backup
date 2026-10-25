package transport

import "syscall"

// processAlive reports whether pid names a live process: one that can be
// opened and whose exit code is still STILL_ACTIVE. A pid that cannot be
// opened has gone (or is not ours, which none of the gate tests' pids are).
func processAlive(pid int) bool {
	const (
		processQueryLimitedInformation = 0x1000
		stillActive                    = 259
	)
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
