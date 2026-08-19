// Command sidesave-cli is the SideSave command-line interface.
package main

import (
	"os"

	"github.com/bytethecookie/sidesave/internal/cliapp"
)

func main() {
	os.Exit(cliapp.Run(os.Args[1:]))
}
