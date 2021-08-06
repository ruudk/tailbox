# Tailbox

Tailbox allows you to tail the output of another process and stream its output in a limited box.

When the program completes, the output is discarded. When the program fails, the whole output is printed.

Tailbox can be used to integrate into other scripts and improve their output.

## Installation

```bash
brew install ruudk/tap/tailbox
```

## Usage
```
usage: tailbox [options] -- <command> [<args>]

Options:
  -failure string
    	Message to print when command failed
  -lines int
    	Number of lines (default 5)
  -running string
    	Message to print while running the command
  -success string
    	Message to print when command finished
```

## Example
```
tailbox -success "✅ Tests passed" -failure "❌ Tests failed" -- vendor/bin/phpunit
```

## Demo

Tailbox being used in a script that iterates over every commit in a branch and performs tests on the commit.

It streams the last 5 lines of the runner. When the tests fail, the whole output is printed.

https://user-images.githubusercontent.com/104180/128398147-d09620ff-b554-48d0-bfcf-bb37df60607a.mp4



