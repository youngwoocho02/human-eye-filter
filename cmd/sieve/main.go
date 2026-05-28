package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"unicode/utf8"

	"github.com/youngwoocho02/token-sieve/internal/sieve"
)

func main() {
	code, err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet("sieve", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := sieve.DefaultOptions()
	flags.IntVar(&options.MaxLines, "max-lines", options.MaxLines, "maximum output lines")
	flags.IntVar(&options.MaxChars, "max-chars", options.MaxChars, "maximum output characters")
	flags.Int64Var(&options.MaxInputBytes, "max-input-bytes", options.MaxInputBytes, "maximum raw input bytes to read before reducing")
	flags.StringVar(&options.Mode, "mode", options.Mode, "reducer mode: auto, text, json, paths, grep, unity, scm")
	flags.StringVar(&options.Tool, "tool", options.Tool, "optional tool hint, such as rg, grep, git, cm, unity-cli, unity-scanner")
	flags.StringVar(&options.Keep, "keep", options.Keep, "priority to keep: error, warning, path, all")
	flags.BoolVar(&options.RawOnFail, "raw-on-fail", options.RawOnFail, "print raw input if reduction fails")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}

	rest := flags.Args()
	if len(rest) > 0 && rest[0] == "run" {
		if len(rest) < 3 || rest[1] != "--" {
			return 2, errors.New("usage: sieve run -- <command> [args...]")
		}

		return runCommand(rest[2:], stdout, stderr, options)
	}

	input, err := readBounded(stdin, options.MaxInputBytes)
	if err != nil {
		return 1, err
	}

	output, err := sieve.Reduce(string(input), options)
	if err != nil {
		if options.RawOnFail {
			_, _ = stdout.Write(input)
			return 0, nil
		}

		return 1, err
	}

	_, err = io.WriteString(stdout, output)
	if err != nil {
		return 1, err
	}

	return 0, nil
}

func runCommand(command []string, stdout, stderr io.Writer, options sieve.Options) (int, error) {
	if options.Tool == "" {
		options.Tool = command[0]
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	out := newBoundedOutput(options.MaxInputBytes)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	combined := out.String()
	reduced, reduceErr := sieve.Reduce(combined, options)
	if reduceErr != nil {
		if options.RawOnFail {
			if _, writeErr := io.WriteString(stdout, combined); writeErr != nil {
				return 1, writeErr
			}
		} else {
			return 1, reduceErr
		}
	} else {
		if _, writeErr := io.WriteString(stdout, reduced); writeErr != nil {
			return 1, writeErr
		}
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, err
		}
	}

	return exitCode, nil
}

func readBounded(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = sieve.DefaultOptions().MaxInputBytes
	}

	limited := io.LimitReader(reader, maxBytes+1)
	input, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(input)) <= maxBytes {
		return input, nil
	}

	input = trimValidUTF8(input[:maxBytes])
	input = append(input, []byte(fmt.Sprintf("\n... <sieve raw input truncated at %d bytes>\n", maxBytes))...)
	return input, nil
}

type boundedOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	maxBytes int64
	trimmed  int64
}

func newBoundedOutput(maxBytes int64) *boundedOutput {
	if maxBytes <= 0 {
		maxBytes = sieve.DefaultOptions().MaxInputBytes
	}
	return &boundedOutput{maxBytes: maxBytes}
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	remaining := b.maxBytes - int64(b.buffer.Len())
	if remaining <= 0 {
		b.trimmed += int64(len(p))
		return len(p), nil
	}

	if int64(len(p)) <= remaining {
		_, _ = b.buffer.Write(p)
		return len(p), nil
	}

	_, _ = b.buffer.Write(trimValidUTF8(p[:remaining]))
	b.trimmed += int64(len(p)) - remaining
	return len(p), nil
}

func trimValidUTF8(input []byte) []byte {
	for len(input) > 0 && !utf8.Valid(input) {
		input = input[:len(input)-1]
	}
	return input
}

func (b *boundedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := trimStringValidUTF8(b.buffer.String())
	if b.trimmed == 0 {
		return text
	}

	return text + fmt.Sprintf("\n... <sieve raw command output truncated; omitted %d bytes>\n", b.trimmed)
}

func trimStringValidUTF8(input string) string {
	for len(input) > 0 && !utf8.ValidString(input) {
		input = input[:len(input)-1]
	}
	return input
}
