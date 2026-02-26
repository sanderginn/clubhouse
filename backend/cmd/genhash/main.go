package main

import (
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		_, _ = os.Stderr.WriteString("Usage: genhash <password>\n")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), 12)
	if err != nil {
		_, _ = os.Stderr.WriteString("Error: " + err.Error() + "\n")
		os.Exit(1)
	}
	_, _ = os.Stdout.Write(append(hash, '\n'))
}
