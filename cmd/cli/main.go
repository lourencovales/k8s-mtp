package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to ascertain home dir")
		os.Exit(1)
	}
	configDir := filepath.Join(home, ".k8s-mtp")
	configPath := filepath.Join(configDir, "config")

	switch args[0] {
	case "login":
		loginCmd(configPath)
	case "tenant":
		tenantCmd(configPath, args[1:])
	case "member":
		memberCmd(configPath, args[1:])
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: \nk8s-mtp login Authenticate with Dex\n")
}
