package cmd

// version is set at build time via ldflags:
//
//	go build -ldflags "-X github.com/hpkotak/shellbud/cmd.version=v0.1.0"
var version = "dev"

func init() {
	rootCmd.Version = version
}
