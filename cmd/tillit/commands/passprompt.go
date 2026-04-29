package commands

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"golang.org/x/term"
)

// passwordReader is the interactive backend used by promptPassword.
// In normal CLI use it reads from /dev/tty without echoing; tests
// swap it for a deterministic stub.
var passwordReader func(prompt string) ([]byte, error) = readPasswordFromTerminal

// promptPassword shows prompt on stderr and returns whatever the
// underlying reader supplies, with trailing CR/LF trimmed. The
// returned slice MUST NOT be retained past the immediate signing
// operation.
func promptPassword(prompt string) ([]byte, error) {
	pw, err := passwordReader(prompt)
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(pw, "\r\n"), nil
}

// promptPasswordTwice asks for the same password twice and verifies
// they match — used at key-creation time to guard against typos.
// An empty response is allowed (the caller decides what to do with
// it: most paths interpret empty as "leave unencrypted").
func promptPasswordTwice(firstPrompt, secondPrompt string) ([]byte, error) {
	a, err := promptPassword(firstPrompt)
	if err != nil {
		return nil, err
	}
	b, err := promptPassword(secondPrompt)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(a, b) {
		return nil, fmt.Errorf("passwords did not match")
	}
	return a, nil
}

// stdinReader is a single bufio.Reader bound to os.Stdin. We can't
// make a fresh one on every call: bufio buffers ahead, so a discarded
// reader takes already-read bytes with it, breaking subsequent reads
// from the same underlying file descriptor.
var stdinReader = bufio.NewReader(os.Stdin)

// readPasswordFromTerminal is the production implementation. It
// prints prompt to stderr, then reads a password from stdin without
// echoing when stdin is a terminal. When stdin is piped (CI / tests
// / scripts) it falls back to a buffered line read.
func readPasswordFromTerminal(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		pw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr) // newline after the silent input
		return pw, err
	}
	return readLine(stdinReader)
}

func readLine(br *bufio.Reader) ([]byte, error) {
	line, err := br.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	return bytes.TrimRight(line, "\r\n"), nil
}
