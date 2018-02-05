package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	appName    = "goiv"
	appVersion = "1.0"
)

func main() {
	flag.Usage = usage

	width := flag.Int("w", 1024, "Window width")
	height := flag.Int("h", 768, "Window height")
	version := flag.Bool("v", false, "Print version and exit")
	filelist := flag.String("f", "", "Use list of images from file, one per line")

	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stdout, "%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	args := arguments(flag.Args())

	if *filelist != "" {
		file, err := os.Open(*filelist)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}

		ln := lines(file)
		args = append(args, ln...)

		file.Close()
	}

	if piped() {
		ln := lines(os.Stdin)
		args = append(args, ln...)
	}

	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	display(args, *width, *height)
}

// usage prints default usage.
func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [FILE1 [FILE2 [...]]]\n", appName)
	fmt.Fprintf(os.Stderr, `
  -f path
	Use list of images from file, one per line
  -w int
	Window width (default 1024)
  -h int
	Window height (default 768)
  -v
	Print version and exit

Keybindings:

  j / Right / PageDown / Space
	Next image

  k / Left / PageUp
	Previous image

  f / F11
	Fullscreen

  [ / ]
	Go 10 images back/forward

  , / .
	Go to first/last image

  q / Escape
	Quit

  Enter
	Print current image path to stdout`)
	fmt.Fprintf(os.Stderr, "\n")
}

// arguments returns slice of arguments.
func arguments(in []string) []string {
	out := make([]string, 0)
	for _, arg := range in {
		if _, err := os.Stat(arg); err == nil {
			out = append(out, arg)
		} else {
			if isURL(arg) {
				out = append(out, arg)
			} else {
				g, err := filepath.Glob(arg)
				if err == nil {
					out = append(out, g...)
				}
			}
		}
	}

	return out
}

// lines returns slice of lines from reader.
func lines(r io.Reader) []string {
	ln := make([]string, 0)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		ln = append(ln, scanner.Text())
	}

	return ln
}

// piped checks if we have a piped stdin.
func piped() bool {
	f, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	if f.Mode()&os.ModeNamedPipe == 0 {
		return false
	} else {
		return true
	}
}

// isURL checks if arguments is URL.
func isURL(arg string) bool {
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return true
	}

	return false
}
