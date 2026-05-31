package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/youngwoocho02/human-eye-filter/internal/humaneye"
)

var Version = "dev"

func main() {
	code, err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) > 0 && args[0] == "setup" {
		return runSetup(args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("hef", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := humaneye.DefaultOptions()
	var showVersion bool
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.IntVar(&options.MaxLines, "max-lines", options.MaxLines, "maximum output lines")
	flags.IntVar(&options.MaxChars, "max-chars", options.MaxChars, "maximum output characters")
	flags.Int64Var(&options.MaxInputBytes, "max-input-bytes", options.MaxInputBytes, "maximum raw input bytes to read before reducing")
	flags.StringVar(&options.Mode, "mode", options.Mode, "reducer mode: auto, text, json, paths, grep, unity, scm")
	flags.StringVar(&options.Tool, "tool", options.Tool, "optional tool hint, such as rg, grep, git, cm, unity-cli, unity-scanner")
	flags.StringVar(&options.Keep, "keep", options.Keep, "priority to keep: error, warning, path, all")
	flags.StringVar(&options.Focus, "focus", options.Focus, "comma-separated keywords to keep before generic samples")
	flags.BoolVar(&options.RawOnFail, "raw-on-fail", options.RawOnFail, "print raw input if reduction fails")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}

	if showVersion {
		_, _ = fmt.Fprintln(stdout, Version)
		return 0, nil
	}

	rest := flags.Args()
	if len(rest) > 0 && rest[0] == "version" {
		_, _ = fmt.Fprintln(stdout, Version)
		return 0, nil
	}

	input, err := readBounded(stdin, options.MaxInputBytes)
	if err != nil {
		return 1, err
	}

	output, err := humaneye.Reduce(string(input), options)
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

func readBounded(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = humaneye.DefaultOptions().MaxInputBytes
	}

	limited := io.LimitReader(reader, maxBytes+1)
	input, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(input)) <= maxBytes {
		return input, nil
	}

	omitted := int64(len(input)) - maxBytes
	buffer := make([]byte, 32*1024)
	for {
		read, readErr := reader.Read(buffer)
		omitted += int64(read)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}

	input = trimValidUTF8(input[:maxBytes])
	input = append(input, []byte(fmt.Sprintf("\n... <hef input limit reached: read %d bytes, omitted %d more raw bytes. Narrow the command and rerun; use more specific paths, globs, filters, or counts.>\n", maxBytes, omitted))...)
	return input, nil
}

func trimValidUTF8(input []byte) []byte {
	for len(input) > 0 && !utf8.Valid(input) {
		input = input[:len(input)-1]
	}
	return input
}
