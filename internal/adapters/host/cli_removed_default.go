//go:build !windows || !tray

package host

import (
	"fmt"
	"os"
)

func refuseRemovedCLI(message string) {
	fmt.Fprintln(os.Stderr, message)
}
