package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/isYaoNoistu/dbbackupctl/internal/cli"
	"github.com/isYaoNoistu/dbbackupctl/internal/exiterr"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.Run(version, commit, date); err != nil {
		var ee *exiterr.ExitError
		if errors.As(err, &ee) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", ee)
			os.Exit(ee.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exiterr.ExitGeneral)
	}
}
