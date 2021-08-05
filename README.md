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
  -success string
    	Message to print when done
```

## Example
```
tailbox -success "✅ Tests passed" -failure "❌ Tests failed" -- vendor/bin/phpunit
```

## Demo
https://user-images.githubusercontent.com/104180/128398147-d09620ff-b554-48d0-bfcf-bb37df60607a.mp4



