package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ruudk/tailbox/tailbox"
)

var numberOfLines int
var headerMessage string
var runningMessage string
var successMessage string
var failureMessage string
var flagset *flag.FlagSet
var version = "dev-main"
var commit string
var date string

func init() {
	flagset = flag.NewFlagSet("tailbox", flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Printf("tailbox %s\n", strings.Trim(strings.Join([]string{version, commit, date}, " "), " "))
		fmt.Println("usage: tailbox [options] -- <command> [<args>]")
		fmt.Println("")
		fmt.Println("Options:")
		flagset.PrintDefaults()
	}
	flagset.IntVar(&numberOfLines, "lines", 6, "Number of lines")
	flagset.StringVar(&headerMessage, "header", "", "Message to print while running the command, when the command finishes or when it fails")
	flagset.StringVar(&runningMessage, "running", "", "Message to print while running the command")
	flagset.StringVar(&successMessage, "success", "", "Message to print when command finished")
	flagset.StringVar(&failureMessage, "failure", "", "Message to print when command failed")
}

func main() {
	err := flagset.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		flagset.Usage()
		os.Exit(0)
	}

	if numberOfLines < 1 {
		flagset.Usage()
		os.Exit(1)
	}

	commandArgs := flagset.Args()
	if len(commandArgs) < 1 {
		flagset.Usage()
		os.Exit(1)
	}

	if headerMessage != "" && (runningMessage != "" || successMessage != "" || failureMessage != "") {
		flagset.Usage()
		fmt.Println()
		fmt.Println("error: cannot use `-header` together with `-running`, `-success` or `-failure`.")
		os.Exit(1)
	}

	tb, err := tailbox.NewTailbox(os.Stdout, numberOfLines, headerMessage, runningMessage, successMessage, failureMessage)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("/bin/sh", "-c", strings.Join(commandArgs, " "))
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	cmd.Stdout = tb
	cmd.Stderr = tb
	err = cmd.Run()
	if err != nil {
		tb.Fail(err)
	} else {
		tb.Success()
	}

	os.Exit(0)
}
