package main

import (
	"os"

	app "github.com/primaprashant/sei/internal"
)

var version = "dev"

func main() {
	os.Exit(app.Run(version, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
