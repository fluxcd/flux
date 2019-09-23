package checkpoint

import (
	"syscall"
)

func getKernelVersion() string {
	v, err := syscall.Sysctl("kern.osrelease")
	if err != nil {
		panic(err)
	}
	return "darwin-" + v
}
