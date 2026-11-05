package main

//go build -o toth.exe main.go
import (
	"os"
	"toth/repl"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			println("\033[1;31mPANIC:", r, "\033[0m")
		}
	}()

	if len(os.Args) != 2 {
		println("\033[0;31mtoth command must have one argument\033[0m")
		return
	}
	if os.Args[1] == "-v" || os.Args[1] == "-version" {
		println("V0.8.0")
		return
	}
	if os.Args[1] == "-info" {
		println("Toth programming language\nCreated by Gionny")
		return
	}

	repl.StartFile(os.Args[1])
}
