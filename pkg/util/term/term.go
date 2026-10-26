package term

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// IsTerminal reports whether stdin is an interactive terminal. False in CI,
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Confirm prints question followed by a y/N prompt and reads one line from
// stdin. Returns true for "y"/"yes" (case-insensitive), false for anything
// else, including empty input and EOF. The default is "no".
//
//	ok := term.Confirm("Push anyway?")
//	if !ok { return fmt.Errorf("aborted") }
func Confirm(question string) (bool, error) {
	return ConfirmFrom(question, os.Stdin, os.Stdout)
}

// ConfirmFrom is Confirm with explicit reader and writer, so it is testable
// without a real terminal.
func ConfirmFrom(question string, in io.Reader, out io.Writer) (bool, error) {
	fmt.Fprintf(out, "%s [y/N]: ", question)

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
