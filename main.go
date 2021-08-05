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

	area := cursor.NewArea()
	full := ""
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	defer cmd.Process.Kill()

	stdout, err := cmd.StdoutPipe()
	defer stdout.Close()
	if err != nil {
		fmt.Printf("failed runnig command: %v", err)
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
			area.Update(full)
			os.Exit(1)
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Printf("failed starting command: %v", err)
		os.Exit(1)
	}

	err = cmd.Wait()
	if err != nil {
		area.Update(full)
		os.Exit(1)
	}

	if successMessage != "" {
		area.Update(successMessage)
	} else {
		area.Clear()
	}

	os.Exit(0)
}