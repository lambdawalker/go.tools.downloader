package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrPromptAborted is returned when the user cancels or aborts interactive prompt input.
var ErrPromptAborted = errors.New("authentication prompt canceled by user")

// Prompter provides an interface for requesting authentication credentials from the user or caller.
type Prompter interface {
	// PromptAuth requests credentials for an unconfigured domain.
	PromptAuth(domain string) (*Entry, error)
}

// TerminalPrompter prompts the user interactively on a terminal using stdin/stdout.
type TerminalPrompter struct {
	in  *bufio.Reader
	out io.Writer
}

// NewTerminalPrompter constructs a prompter reading from standard input and writing to standard output.
func NewTerminalPrompter() *TerminalPrompter {
	return &TerminalPrompter{
		in:  bufio.NewReader(os.Stdin),
		out: os.Stdout,
	}
}

// NewCustomTerminalPrompter allows providing custom reader and writer streams (useful for testing).
func NewCustomTerminalPrompter(in io.Reader, out io.Writer) *TerminalPrompter {
	return &TerminalPrompter{
		in:  bufio.NewReader(in),
		out: out,
	}
}

// PromptAuth displays an interactive terminal menu asking the user for authentication method and credentials.
func (p *TerminalPrompter) PromptAuth(domain string) (*Entry, error) {
	_, _ = fmt.Fprintf(p.out, "\n[Auth] Authentication configuration required for %q\n", domain)
	_, _ = fmt.Fprintln(p.out, "1. None (Anonymous / Public)")
	_, _ = fmt.Fprintln(p.out, "2. Bearer Token (token or env:VAR)")
	_, _ = fmt.Fprintln(p.out, "3. Basic Auth (username & password or env:VAR)")
	_, _ = fmt.Fprintln(p.out, "4. Custom HTTP Header (e.g. X-API-Key or env:VAR)")
	_, _ = fmt.Fprint(p.out, "Select authentication method [1-4] (default 1): ")

	line, err := p.readLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			// In non-interactive or redirected stdin, default to None
			return &Entry{Method: MethodNone}, nil
		}
		return nil, err
	}

	choice := strings.TrimSpace(line)
	switch choice {
	case "2":
		_, _ = fmt.Fprint(p.out, "Enter Bearer Token (or env:VAR_NAME): ")
		token, err := p.readLine()
		if err != nil {
			return nil, err
		}
		return &Entry{
			Method: MethodBearer,
			Token:  strings.TrimSpace(token),
		}, nil

	case "3":
		_, _ = fmt.Fprint(p.out, "Enter Username: ")
		user, err := p.readLine()
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprint(p.out, "Enter Password (or env:VAR_NAME): ")
		pass, err := p.readLine()
		if err != nil {
			return nil, err
		}
		return &Entry{
			Method:   MethodBasic,
			Username: strings.TrimSpace(user),
			Password: strings.TrimSpace(pass),
		}, nil

	case "4":
		_, _ = fmt.Fprint(p.out, "Enter Header Name (e.g. X-API-Key): ")
		hKey, err := p.readLine()
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprint(p.out, "Enter Header Value (or env:VAR_NAME): ")
		hVal, err := p.readLine()
		if err != nil {
			return nil, err
		}
		headers := map[string]string{
			strings.TrimSpace(hKey): strings.TrimSpace(hVal),
		}
		return &Entry{
			Method:  MethodHeader,
			Headers: headers,
		}, nil

	default: // "1" or empty
		return &Entry{Method: MethodNone}, nil
	}
}

func (p *TerminalPrompter) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// CallbackPrompter delegates prompting to a user-supplied function.
type CallbackPrompter struct {
	fn func(domain string) (*Entry, error)
}

// NewCallbackPrompter wraps a callback function into a Prompter.
func NewCallbackPrompter(fn func(domain string) (*Entry, error)) *CallbackPrompter {
	return &CallbackPrompter{fn: fn}
}

// PromptAuth invokes the configured callback function.
func (c *CallbackPrompter) PromptAuth(domain string) (*Entry, error) {
	if c.fn == nil {
		return &Entry{Method: MethodNone}, nil
	}
	return c.fn(domain)
}

// NonInteractivePrompter provides automatic anonymous fallback without prompting.
type NonInteractivePrompter struct{}

// NewNonInteractivePrompter creates a prompter that defaults to MethodNone.
func NewNonInteractivePrompter() *NonInteractivePrompter {
	return &NonInteractivePrompter{}
}

// PromptAuth returns MethodNone immediately.
func (n *NonInteractivePrompter) PromptAuth(_ string) (*Entry, error) {
	return &Entry{Method: MethodNone}, nil
}
