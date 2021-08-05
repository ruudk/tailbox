package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/atomicgo/cursor"
)

var numberOfLines int
var successMessage string
var failureMessage string
var flagset *flag.FlagSet

func init() {
	flagset = flag.NewFlagSet("tailbox", flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Println("usage: tailbox [options] -- <command> [<args>]")
		fmt.Println("")
		fmt.Println("Options:")
		flagset.PrintDefaults()
	}
	flagset.IntVar(&numberOfLines, "lines", 5, "Number of lines")
	flagset.StringVar(&successMessage, "success", "", "Message to print when done")
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

	if failureMessage != "" {
		failureMessage += "\n"
	}

	area := cursor.NewArea()
	full := ""
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	defer func() {
		err = cmd.Process.Kill()
		if err != nil {
			fmt.Printf("failed killing command: %v\n", err)
			os.Exit(1)
		}
	}()

	stdout, err := cmd.StdoutPipe()
	defer func() {
		err = stdout.Close()
		if err != nil {
			fmt.Printf("failed closing stdout: %v\n", err)
			os.Exit(1)
		}
	}()
	if err != nil {
		fmt.Printf("failed runnig command: %v\n", err)
		os.Exit(1)
	}

	go func() {
		s := bufio.NewScanner(stdout)
		s.Split(bufio.ScanRunes)

		box := ""
		lines := 0
		for s.Scan() {
			full += s.Text()
			box += s.Text()

			if s.Text() == "\n" {
				lines++

				if lines > numberOfLines {
					box = strings.Join(strings.Split(box, "\n")[1:], "\n")
					lines--
				}
			}

			area.Update(box)
		}

		if err := s.Err(); err != nil {
			area.Update(failureMessage + full)
			os.Exit(1)
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Printf("failed starting command: %v\n", err)
		os.Exit(1)
	}

	err = cmd.Wait()
	if err != nil {
		area.Update(failureMessage + full)
		os.Exit(1)
	}

	if successMessage != "" {
		area.Update(successMessage)
	} else {
		area.Clear()
	}

	os.Exit(0)
}
