package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/atomicgo/cursor"
)

var numberOfLines int
var runningMessage string
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

	if failureMessage != "" {
		failureMessage += "\n"
	}

	area := cursor.NewArea()

	if runningMessage != "" {
		area.Update(runningMessage)
	}

	full := ""
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	defer func() {
		err = cmd.Process.Kill()
		if err != nil {
			fmt.Printf("failed killing command: %v\n", err)
			os.Exit(1)
		}
	}()

	stdout, err := cmd.StdoutPipe()
	defer func() {
		_ = stdout.Close()
	}()
	if err != nil {
		fmt.Printf("failed runnig command: %v\n", err)
		os.Exit(1)
	}

	stderr, err := cmd.StderrPipe()
	defer func() {
		_ = stderr.Close()
	}()
	if err != nil {
		fmt.Printf("failed runnig command: %v\n", err)
		os.Exit(1)
	}

	cursor.Hide()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		s := bufio.NewScanner(io.MultiReader(stdout, stderr))
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
	}()

	err = cmd.Start()
	if err != nil {
		cursor.Show()
		fmt.Printf("failed starting command: %v\n", err)
		os.Exit(1)
	}

	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		cursor.Show()
		area.Update(failureMessage + full)
		os.Exit(1)
	}

	cursor.Show()
	if successMessage != "" {
		area.Update(successMessage)
	} else {
		area.Clear()
	}

	os.Exit(0)
}
