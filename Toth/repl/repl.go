package repl

//go run main.go "file.th"
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"toth/evaluator"
	"toth/lexer"
	"toth/object"
	"toth/parser"
)

const (
	colorReset         = "\033[0m"
	colorBlack         = "\033[0;30m"
	colorRed           = "\033[0;31m"
	colorGreen         = "\033[0;32m"
	colorYellow        = "\033[0;33m"
	colorBlue          = "\033[0;34m"
	colorMagenta       = "\033[0;35m"
	colorCyan          = "\033[0;36m"
	colorWhite         = "\033[0;37m"
	colorBrightBlack   = "\033[1;30m"
	colorBrightRed     = "\033[1;31m"
	colorBrightGreen   = "\033[1;32m"
	colorBrightYellow  = "\033[1;33m"
	colorBrightBlue    = "\033[1;34m"
	colorBrightMagenta = "\033[1;35m"
	colorBrightCyan    = "\033[1;36m"
	colorBrightWhite   = "\033[1;37m"
)

func Start(in string, out io.Writer) {
	env := object.NewEnvironment()
	fmt.Fprint(out)

	line := in
	l := lexer.New(line)
	p := parser.New(l)

	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors())
		return
	}

	evaluated := evaluator.Eval(program, env)
	if evaluated != nil {
		if evaluated.Type() == object.ERROR_OBJ {
			io.WriteString(out, evaluated.Inspect())
		}
		io.WriteString(out, "")
	}
}

func StartFile(arg string) {
	filePath := arg

	if len(arg) < 3 || string(arg[len(arg)-3:]) != ".th" {
		fmt.Fprintf(os.Stderr, colorRed+"Error file extension: file must end with .th\n"+colorReset)
		return
	}

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		fmt.Fprintf(os.Stderr, colorRed+"Error opening file: %v\n"+colorReset, err)
		return
	}

	contentWithoutNewlines := string(data)
	Start(contentWithoutNewlines, os.Stdout)
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, colorRed+" Toth: the program ran into some problem!\n"+colorReset)
	for _, msg := range errors {
		io.WriteString(out, colorRed+"\t"+msg+"\n"+colorReset)
	}
}
