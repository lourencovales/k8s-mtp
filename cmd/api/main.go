package main

import (
	"flag"
	"fmt"
	"os"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
)

func main() {
	configFile := flag.String("config", "config.json", "config file")
	debug := flag.Bool("debug", false, "enable debug mode")
	flag.Parse()

	_ = configFile
	_ = debug

	cfg, err := config.Load(*configFile) 
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(cfg)
}
