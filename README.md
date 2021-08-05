# Tailbox

Tailbox allows you to tail the output of another process and stream its output in a limited box.

When the program completes, the output is discarded. When the program fails, the whole output is printed.

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
tailbox -success "✅ Unit tests passed" -failure "❌ Unit tests failed" -- vendor/bin/phpunit
```
