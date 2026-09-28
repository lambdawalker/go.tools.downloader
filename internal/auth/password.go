package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PromptPassword prompts the user in the terminal for a password without echoing input.
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	stdinFd := int(os.Stdin.Fd())

	if term.IsTerminal(stdinFd) {
		bytePassword, err := term.ReadPassword(stdinFd)
		fmt.Println() // print newline after user presses Enter
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePassword)), nil
	}

	// Non-terminal fallback (e.g. piped stdin)
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

// ConfirmPassword prompts for a password and asks to re-type it, verifying they match.
func ConfirmPassword(prompt string) (string, error) {
	p1, err := PromptPassword(prompt)
	if err != nil {
		return "", err
	}
	if p1 == "" {
		return "", errors.New("password cannot be empty")
	}

	p2, err := PromptPassword("Confirm Master Password: ")
	if err != nil {
		return "", err
	}

	if p1 != p2 {
		return "", errors.New("passwords do not match")
	}

	return p1, nil
}

// ReadPasswordFromFile reads a password string from the specified file path.
func ReadPasswordFromFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed reading password file: %w", err)
	}
	pass := strings.TrimRight(string(data), "\r\n")
	if pass == "" {
		return "", errors.New("password file is empty")
	}
	return pass, nil
}

// ReadPasswordFromStdin reads a single password line directly from standard input.
func ReadPasswordFromStdin() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed reading password from stdin: %w", err)
	}
	pass := strings.TrimRight(text, "\r\n")
	if pass == "" {
		return "", errors.New("received empty password from stdin")
	}
	return pass, nil
}
