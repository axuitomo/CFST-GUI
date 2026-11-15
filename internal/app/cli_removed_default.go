//go:build !windows || !tray

package app

import (
	"fmt"
	"os"
)

func refuseRemovedCLI(message string) {
	fmt.Fprintln(os.Stderr, message)
}
