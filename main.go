package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ruudk/tailbox/tailbox"
	"github.com/ruudk/tailbox/tailbox/screen"
)

var version = "dev-main"
var commit string
var date string

func main() {
	path, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get current working directory: %s", err)
	}

	file := filepath.Join(path, "tailbox.hcl")
	if len(os.Args) > 1 {
		file = filepath.Join(path, os.Args[1])
	}
	log.Printf("Using config file %s", file)
	config, err := tailbox.NewConfig(file)
	if err != nil {
		log.Fatal(err)
	}

	display, err := screen.NewDisplay(os.Stdout, config.Defaults.Lines)
	if err != nil {
		log.Fatal(err)
	}

	tb, err := tailbox.NewTailbox(display, config)
	if err != nil {
		log.Fatal(err)
	}

	tb.Start()

	os.Exit(0)
}
