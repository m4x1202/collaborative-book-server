package main

import (
	"os"

	"github.com/m4x1202/collaborative-book/internal/app/cmd"
)

func main() {
	os.Exit(cmd.Run())
}
