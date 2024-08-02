package main

import (
	"context"

	"github.com/machbase/neo-pkgdev/cmd/pkgdev"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(pkgdev.NewCmd().ExecuteContext(context.Background()))
}
