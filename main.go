package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const version = "0.1.1"

const help = `Electric Shell

Usage:
  electric-shell                    Wrap $SHELL in an Electric-colored PTY
  electric-shell -c <command>       Run one command
  command | electric-shell          Run commands read from stdin

Options:
  -c, --command <command>           Command to run
  -s, --shell <path>                Shell to wrap (defaults to $SHELL)
  --color                           Force Electric colors
  --no-color                        Disable Electric colors
  -h, --help                        Show this help
  -v, --version                     Show the version`

type options struct {
	colorMode int
	command   string
	shell     string
	help      bool
	version   bool
}

func parseArgs(arguments []string) (options, error) {
	result := options{colorMode: -1, shell: os.Getenv("SHELL")}
	if result.shell == "" {
		result.shell = "/bin/sh"
	}
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--color":
			result.colorMode = 1
		case "--no-color":
			result.colorMode = 0
		case "-h", "--help":
			result.help = true
		case "-v", "--version":
			result.version = true
		case "-s", "--shell":
			index++
			if index >= len(arguments) {
				return result, fmt.Errorf("%s requires a path", arguments[index-1])
			}
			result.shell = arguments[index]
		case "-c", "--command":
			index++
			if index >= len(arguments) {
				return result, fmt.Errorf("%s requires a command", arguments[index-1])
			}
			result.command = strings.Join(arguments[index:], " ")
			return result, nil
		default:
			return result, fmt.Errorf("unknown option: %s", arguments[index])
		}
	}
	return result, nil
}

func usesColor(mode int, output *os.File) bool {
	if mode >= 0 {
		return mode == 1
	}
	_, disabled := os.LookupEnv("NO_COLOR")
	return term.IsTerminal(int(output.Fd())) && !disabled
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 1
}

func runCommand(shell string, command string, color bool) int {
	child := exec.Command(shell, "-c", command)
	child.Stdin = os.Stdin
	stdout := newColorWriter(os.Stdout, color)
	stderr := newColorWriter(os.Stderr, color)
	child.Stdout = stdout
	child.Stderr = stderr
	err := child.Run()
	_ = stdout.flush()
	_ = stderr.flush()
	return exitCode(err)
}

func runInteractive(shell string, color bool) int {
	child := exec.Command(shell)
	child.Env = os.Environ()

	size, _ := pty.GetsizeFull(os.Stdin)
	terminal, err := pty.StartWithSize(child, size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "electric-shell: %v\n", err)
		return 1
	}
	defer terminal.Close()

	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "electric-shell: %v\n", err)
		return 1
	}
	defer term.Restore(int(os.Stdin.Fd()), state)

	resizes := make(chan os.Signal, 1)
	signal.Notify(resizes, syscall.SIGWINCH)
	defer signal.Stop(resizes)
	go func() {
		for range resizes {
			_ = pty.InheritSize(os.Stdin, terminal)
		}
	}()
	resizes <- syscall.SIGWINCH

	go func() {
		_, _ = io.Copy(terminal, os.Stdin)
	}()
	output := newColorWriter(os.Stdout, color)
	_, _ = io.Copy(output, terminal)
	_ = output.flush()
	return exitCode(child.Wait())
}

func run(arguments []string) int {
	configuration, err := parseArgs(arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "electric-shell: %v\nTry electric-shell --help.\n", err)
		return 2
	}
	if configuration.help {
		fmt.Println(help)
		return 0
	}
	if configuration.version {
		fmt.Println(version)
		return 0
	}

	color := usesColor(configuration.colorMode, os.Stdout)
	if configuration.command != "" {
		return runCommand(configuration.shell, configuration.command, color)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		input, readError := io.ReadAll(os.Stdin)
		if readError != nil {
			fmt.Fprintf(os.Stderr, "electric-shell: %v\n", readError)
			return 1
		}
		if strings.TrimSpace(string(input)) == "" {
			return 0
		}
		return runCommand(configuration.shell, string(input), color)
	}
	return runInteractive(configuration.shell, color)
}

func main() {
	os.Exit(run(os.Args[1:]))
}
