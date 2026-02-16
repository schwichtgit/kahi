package main

import (
	"fmt"
	"runtime"

	"github.com/kahidev/kahi/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		goVer := version.GoVersion
		if goVer == "" {
			goVer = runtime.Version()
		}
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "kahi %s\n", version.Version)
		fmt.Fprintf(w, "  commit:  %s\n", version.Commit)
		fmt.Fprintf(w, "  built:   %s\n", version.Date)
		fmt.Fprintf(w, "  go:      %s\n", goVer)
		fmt.Fprintf(w, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(w, "  fips:    %s\n", version.FIPS)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
