package evaluator

import (
	"bufio"
	"fmt"
	"image/png"
	"math/rand/v2"
	"os"
	"syscall"
	"time"
	"toth/object"
	"unicode/utf8"
	"unsafe"
)

var curEnv object.Environment

func argumentTypeError(funcNum int, funcname string, expectedType string, gotType string) object.Object {
	if funcNum == -1 {
		return newError("argument to `%s` must be %s, got %s", funcname, expectedType, gotType)
	}
	return newError("argument[%d] to `%s` must be %s, got %s", funcNum, funcname, expectedType, gotType)
}

var builtins = map[string]*object.Builtin{
	"bytelen": { //@audit-info bytelen
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'bytelen' wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Array:
				return &object.Integer{Value: int64(len(arg.Elements))}
			case *object.String:
				return &object.Integer{Value: int64(len(arg.Value))}
			case *object.Hash:
				return &object.Integer{Value: int64(len(arg.Pairs))}
			default:
				return newError("argument to `bytelen` not supported, got %s", args[0].Type())
			}

		},
	},
	"len": { //@audit-info len
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'len' wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Array:
				return &object.Integer{Value: int64(len(arg.Elements))}
			case *object.String:
				count := 0
				for range arg.Value {
					count++
				}
				return &object.Integer{Value: int64(count)}
			case *object.Hash:
				return &object.Integer{Value: int64(len(arg.Pairs))}
			default:
				return newError("argument to `len` not supported, got %s", args[0].Type())
			}

		},
	},
	"first": { //@audit-info first
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'first' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return argumentTypeError(-1, "first", "ARRAY", string(args[0].Type()))
			}
			arr := args[0].(*object.Array)
			if len(arr.Elements) > 0 {
				return arr.Elements[0]
			}
			return NULL
		},
	},
	"last": { //@audit-info last
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'last' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument `last` must be ARRAY, got %s", args[0].Type())
			}
			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				return arr.Elements[length-1]
			}
			return NULL
		},
	},
	"shift": { //@audit-info shift
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'shift' wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `shift` must be ARRAY, got %s",
					args[0].Type())
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				newElements := make([]object.Object, length-1)
				copy(newElements, arr.Elements[1:length])
				return &object.Array{Elements: newElements}
			}

			return NULL
		},
	},
	"push": { //@audit-info push
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'push' wrong number of arguments. got=%d, want=2",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `push` must be ARRAY, got %s",
					args[0].Type())
			}
			arr := args[0].(*object.Array)
			length := len(arr.Elements)

			newElements := make([]object.Object, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]
			return &object.Array{Elements: newElements}
		},
	},
	"pushAt": { //@audit-info pushAt
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'pushAt' wrong number of arguments. got=%d, want=2",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument[0] to `pushAt` must be ARRAY, got %s",
					args[0].Type())
			}
			if args[2].Type() != object.INTEGER_OBJ {
				return newError("argument[2] to `pushAt` must be INTEGER, got %s",
					args[2].Type())
			}
			arr := args[0].(*object.Array).Elements
			elem := args[1]
			index := args[2].(*object.Integer).Value
			length := len(arr)
			if index > int64(length) {
				return newError("'pushAt' argument[2] index out of range")
			}

			newArr := make([]object.Object, length+1)
			copy(newArr, arr)
			for i := 0; i < length; i++ {
				if i >= int(index) {
					newArr[i+1] = arr[i]
				}
			}
			newArr[index] = elem

			return &object.Array{Elements: newArr}
		},
	},
	"println": { //@audit-info println
		Fn: func(args ...object.Object) object.Object {
			for n, arg := range args {
				s := arg.Inspect()
				var result string = s
				if n > 0 {
					fmt.Print(" " + result)
				} else {
					fmt.Print(result)
				}
			}
			fmt.Print("\n")
			return NULL
		},
	},
	"print": { //@audit-info print
		Fn: func(args ...object.Object) object.Object {
			for _, arg := range args {
				fmt.Print(arg.Inspect())
			}

			return NULL
		},
	},
	"printPNG": { //@audit-info printPNG
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'printPNG' wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to `printPNG` must be STRING, got %s",
					args[0].Type())
			}

			file, err := os.Open(args[0].Inspect())
			if err != nil {
				return newError("'printPNG' %s", err)
			}
			defer file.Close()
			println(file.Name())
			img, err := png.Decode(file)
			if err != nil {
				return newError("'printPNG' %s", err)
			}

			oldBounds := img.Bounds()
			oldWidth := oldBounds.Dx()
			oldHeight := oldBounds.Dy()
			newWidth := 32
			newHeight := 32

			xScale := float64(oldWidth) / float64(newWidth)
			yScale := float64(oldHeight) / float64(newHeight)
			for y := 0; y < newHeight; y++ {
				for x := 0; x < newWidth; x++ {
					srcX := int(float64(x) * xScale)
					srcY := int(float64(y) * yScale)
					r, g, b, a := img.At(srcX, srcY).RGBA()
					if a < 255/2 {
						fmt.Printf("\033[38;2;%d;%d;%dm%s\033[0m", r>>8, g>>8, b>>8, "   ")
					} else {
						fmt.Printf("\033[38;2;%d;%d;%dm%s\033[0m", r>>8, g>>8, b>>8, "███")
					}
				}
				fmt.Printf("\n")
			}

			return NULL
		},
	},
	"CLRprintln": { //@audit-info CLRprintln
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 4 {
				return newError("'CLRprintln' wrong number of arguments. got=%d, want=4",
					len(args))
			}

			if args[1].Type() != object.INTEGER_OBJ {
				return newError("argument[1] to `CLRprintln` must be INTEGER, got %s",
					args[0].Type())
			}
			if args[2].Type() != object.INTEGER_OBJ {
				return newError("argument[2] to `CLRprintln` must be INTEGER, got %s",
					args[0].Type())
			}
			if args[3].Type() != object.INTEGER_OBJ {
				return newError("argument[3] to `CLRprintln` must be INTEGER, got %s",
					args[0].Type())
			}
			r := args[1].(*object.Integer).Value
			g := args[2].(*object.Integer).Value
			b := args[3].(*object.Integer).Value
			if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
				return newError("`CLRprintln` Invalid Colorcode: must be between 0 and 255. got=%s", args[1].Inspect())
			}
			fmt.Printf("\033[38;2;%d;%d;%dm%s\033[0m\n", r, g, b, args[0].Inspect())

			return NULL
		},
	},
	"CLRprint": { //@audit-info CLRprint
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 4 {
				return newError("'CLRprint' wrong number of arguments. got=%d, want=4",
					len(args))
			}

			if args[1].Type() != object.INTEGER_OBJ {
				return newError("argument[1] to `CLRprint` must be INTEGER, got %s",
					args[0].Type())
			}
			if args[2].Type() != object.INTEGER_OBJ {
				return newError("argument[2] to `CLRprint` must be INTEGER, got %s",
					args[0].Type())
			}
			if args[3].Type() != object.INTEGER_OBJ {
				return newError("argument[3] to `CLRprint` must be INTEGER, got %s",
					args[0].Type())
			}
			r := args[1].(*object.Integer).Value
			g := args[2].(*object.Integer).Value
			b := args[3].(*object.Integer).Value
			if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
				return newError("`CLRprint` Invalid Colorcode: must be between 0 and 255. got=%s", args[1].Inspect())
			}
			fmt.Printf("\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, args[0].(*object.String).Value)

			return NULL
		},
	},

	"pop": { //@audit-info pop
		Fn: func(args ...object.Object) object.Object {

			switch args[0].(type) {
			case *object.Array:
				if len(args) != 2 {
					return newError("'pop' wrong number of arguments. got=%d, want=2",
						len(args))
				}
				if args[1].Type() != object.INTEGER_OBJ {
					return newError("argument[1] to `pop` must be INTEGER, got %s",
						args[0].Type())
				}

				arr := args[0].(*object.Array)
				i := args[1].(*object.Integer).Value
				length := len(arr.Elements)
				if length > 0 && i < int64(length) && i > -1 {
					newElements := make([]object.Object, length-1)
					copy(newElements, arr.Elements[:i])
					copy(newElements[i:], arr.Elements[i+1:])
					return &object.Array{Elements: newElements}

				}
			case *object.Hash:
				if len(args) != 2 {
					return newError("'hashEdit' wrong number of arguments. got=%d, want=3",
						len(args))
				}
				if args[1].Type() != object.STRING_OBJ {
					return newError("argument[1] `hashEdit` must be STRING, got %s", args[1].Type())
				}

				hash := args[0].(*object.Hash).Pairs
				target := args[1].(*object.String)

				var newhash map[object.HashKey]object.HashPair = hash

				delete(newhash, target.HashKey())

				return &object.Hash{Pairs: newhash}
			default:
				return newError("argument[0] `hashEdit` must be ARRAY/HASH, got %s", args[0].Type())
			}
			return NULL
		},
	},
	"charAt": { //@audit-info charAt
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'charAt' wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `charAt` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.INTEGER_OBJ {
				return newError("argument[1] `charAt` must be INTEGER, got %s", args[1].Type())
			}

			str := args[0].(*object.String).Value
			length := len(str)
			index := args[1].(*object.Integer).Value
			if index < int64(length) && index > -1 {
				return &object.String{Value: string([]rune(str)[index])}
			}

			return NULL
		},
	},
	"randomInt": { //@audit-info randomInt
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'randomInt' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.INTEGER_OBJ {
				return newError("argument[0] `first` must be INTEGER, got %s", args[0].Type())
			}
			if args[1].Type() != object.INTEGER_OBJ {
				return newError("argument[1] `first` must be INTEGER, got %s", args[1].Type())
			}

			val1 := args[0].(*object.Integer).Value
			val2 := args[1].(*object.Integer).Value

			return &object.Integer{Value: rand.Int64N(val2-val1) + val1}
		},
	},

	"toString": { //@audit-info toString
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'toString' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() == object.ERROR_OBJ || args[0].Type() == object.FUNCTION_OBJ || args[0].Type() == object.NULL_OBJ || args[0].Type() == object.RETURN_VALUE_OBJ {
				return newError("argument `toString` cannot be %s", args[0].Type())
			}
			converted := args[0].Inspect()

			return &object.String{Value: converted}
		},
	},

	"toInt": { //@audit-info toInt
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'toInt' wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Float:
				return &object.Integer{Value: int64(arg.Value)}
			case *object.String:
				isNegative := false
				finalnum := 0

				if arg.Value[0] == '-' {
					arg.Value = arg.Value[1:]
					isNegative = true
				}
				for _, ch := range arg.Value {
					if '0' <= ch && ch <= '9' {
						finalnum = finalnum*10 + int(ch-'0')
					} else {
						return NULL
					}
				}
				if isNegative {
					finalnum *= -1
				}

				return &object.Integer{Value: int64(finalnum)}

			default:
				return newError("argument to `toInt` not supported, got %s", args[0].Type())
			}
		},
	},
	"replace": { //@audit-info replace
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'replace' wrong number of arguments. got=%d, want=3", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `replace` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `replace` must be STRING, got %s", args[1].Type())
			}
			if args[2].Type() != object.STRING_OBJ {
				return newError("argument[2] `replace` must be STRING, got %s", args[2].Type())
			}
			str := args[0].(*object.String).Value
			replaceWord := args[1].(*object.String).Value
			replaceTarget := args[2].(*object.String).Value
			var finalStr string
			if replaceWord == "" {
				return newError("argument[1] is empty")
			}
			start := 0
			for {
				index := -1
				for i := 0; i <= len(str[start:])-len(replaceWord); i++ {
					if str[start+i:start+i+len(replaceWord)] == replaceWord {
						index = i
						break
					}
				}
				if index == -1 {
					finalStr += str[start:]
					break
				}
				finalStr += str[start:start+index] + replaceTarget
				start += index + len(replaceWord)
			}

			return &object.String{Value: finalStr}
		},
	},
	"split": { //@audit-info split
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'split' wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `split` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ && args[1].Type() != object.ARRAY_OBJ {
				return newError("argument[1] `split` must be STRING/ARRAY, got %s", args[1].Type())
			}
			var newArr []object.Object
			strArr := args[0].(*object.String).Value
			var splitterList []string

			switch args[1].(type) { //split type check
			case *object.String:
				arg2 := args[1].(*object.String).Value
				splitterList = append(splitterList, arg2)

			case *object.Array:
				arg2 := args[1].(*object.Array).Elements
				for _, el := range arg2 {
					if el.Type() != object.STRING_OBJ {
						return newError("argument[1] support only arrays of STRING, '%s'=%s", el.Inspect(), el.Type())
					}
					splitterList = append(splitterList, el.(*object.String).Value)
				}
			}

			splitter := splitterList[0]

			if splitter == "" {
				return newError("argument[1] is empty")
			}

			start := 0
			end := 0
			for i := range strArr {
				if end < len(splitter) && strArr[i] == splitter[end] {
					end++
					if end == len(splitter) {
						newArr = append(newArr, &object.String{Value: strArr[start : i-len(splitter)+1]})
						start = i + 1
						end = 0
					}
				} else {
					if end > 0 {
						end = 0
						i = start
						start = i + 1
					}
				}
			}
			if start < len(strArr) {
				newArr = append(newArr, &object.String{Value: strArr[start:]})
			}

			return &object.Array{Elements: newArr}

		},
	},

	"splitKeep": { //@audit-info splitKeep
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'splitKeep' wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `splitKeep` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `splitKeep` must be STRING, got %s", args[1].Type())
			}
			var newArr []object.Object
			strArr := args[0].(*object.String).Value
			splitter := args[1].(*object.String).Value

			if splitter == "" {
				return newError("'splitKeep' argument[1] is empty")
			}

			start := 0
			end := 0
			for i := range strArr {
				if end < len(splitter) && strArr[i] == splitter[end] {
					end++
					if end == len(splitter) {
						newArr = append(newArr, &object.String{Value: strArr[start : i-len(splitter)+1]})
						newArr = append(newArr, &object.String{Value: splitter})
						start = i + 1
						end = 0
					}
				} else {
					if end > 0 {
						end = 0
						i = start
						start = i + 1
					}
				}
			}
			if start < len(strArr) {
				newArr = append(newArr, &object.String{Value: strArr[start:]})
			}

			return &object.Array{Elements: newArr}

		},
	},

	"substring": { //@audit-info substring
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'split' wrong number of arguments. got=%d, want=3", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `substring` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.INTEGER_OBJ {
				return newError("argument[1] `substring` must be INTEGER, got %s", args[1].Type())
			}
			if args[2].Type() != object.INTEGER_OBJ {
				return newError("argument[2] `substring` must be INTEGER, got %s", args[1].Type())
			}
			str := args[0].(*object.String).Value
			id1 := args[1].(*object.Integer).Value
			id2 := args[2].(*object.Integer).Value

			if id1 < 0 || id2 > int64(len(str)) || id1 > id2 {
				return newError("`substring` arguments are not valid: %d, %d", id1, id2)
			}

			return &object.String{Value: str[id1:id2]}
		},
	},
	"insert": { //@audit-info insert
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'insert' wrong number of arguments. got=%d, want=3", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `insert` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `insert` must be STRING, got %s", args[1].Type())
			}
			if args[2].Type() != object.INTEGER_OBJ {
				return newError("argument[2] `insert` must be INTEGER, got %s", args[2].Type())
			}

			str := args[0].(*object.String).Value
			strPart := args[1].(*object.String).Value
			index := args[2].(*object.Integer).Value

			final := str[:index] + strPart + str[index:]

			return &object.String{Value: final}
		},
	},
	"typeof": { //@audit-info typeof
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'typeof' wrong number of arguments. got=%d, want=1", len(args))
			}

			return &object.String{Value: string(args[0].Type())}
		},
	},
	"bitlength": { //@audit-info bitlength
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'bitlength' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.INTEGER_OBJ {
				return newError("argument `bitlength` must be INTEGER, got %s", args[0].Type())
			}
			v := args[0].(*object.Integer).Value

			if v < 0 {
				v = -v
			}
			length := 0
			for v > 0 {
				length++
				v >>= 1
			}

			return &object.Integer{Value: int64(length)}
		},
	},
	"throwError": { //@audit-info throwError
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'throwError' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `throwError` must be STRING, got %s", args[0].Type())
			}
			err := args[0].(*object.String).Value

			return newError("throwed error: %s", err)
		},
	},
	"readln": { //@audit-info readln
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 0 {
				return newError("'readln' wrong number of arguments. got=%d, want=0", len(args))
			}

			reader := bufio.NewReader(os.Stdin)
			in, err := reader.ReadString('\n')
			if err != nil {
				return NULL
			}
			var finalstr string = in[:len(in)-2]
			return &object.String{Value: finalstr}
		},
	},
	"join": { //@audit-info join
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 && len(args) != 2 {
				return newError("'join' wrong number of arguments. got=%d, want=1/2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ && args[0].Type() != object.HASH_OBJ {
				return newError("argument[0] `join` must be ARRAY/HASH, got %s", args[0].Type())
			}

			final := ""
			symbol := ""
			if len(args) == 2 {
				if args[1].Type() != object.STRING_OBJ {
					return newError("argument[1] `join` must be STRING, got %s", args[1].Type())
				}
				symbol = args[1].(*object.String).Value
			}

			if args[0].Type() == object.ARRAY_OBJ {
				for i, elem := range args[0].(*object.Array).Elements {
					final += elem.Inspect()
					if i != len(args[0].(*object.Array).Elements)-1 {
						final += symbol
					}
				}
			} else {
				i := 0
				for _, pair := range args[0].(*object.Hash).Pairs {
					i++
					final += pair.Key.Inspect() + symbol + pair.Value.Inspect()
					if i < len(args[0].(*object.Hash).Pairs) {
						final += symbol
					}
				}
			}

			return &object.String{Value: final}
		},
	},
	"toUpper": { //@audit-info toUpper
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return newError("'toUpper' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `toUpper` must be STRING, got %s", args[0].Type())
			}
			final := ""
			str := args[0].(*object.String).Value
			for _, c := range str {
				if c >= 97 && c <= 122 {
					final += string(rune(c - 32))

				} else {
					final += string(c)
				}
			}
			return &object.String{Value: final}
		},
	},
	"toLower": { //@audit-info toLower
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return newError("'toLower' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `toLower` must be STRING, got %s", args[0].Type())
			}
			final := ""
			str := args[0].(*object.String).Value
			for _, c := range str {
				if c >= 65 && c <= 90 {
					final += string(rune(c + 32))

				} else {
					final += string(c)
				}
			}
			return &object.String{Value: final}
		},
	},
	"createDir": { //@audit-info createDir
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return newError("'createDir' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `createDir` must be STRING, got %s", args[0].Type())
			}

			str := args[0].(*object.String).Value

			os.MkdirAll(str, os.ModePerm)
			return NULL
		},
	},
	"createFile": { //@audit-info createFile
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return newError("'createFile' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `createFile` must be STRING, got %s", args[0].Type())
			}

			str := args[0].(*object.String).Value

			file, err := os.Create(str)
			if err != nil {
				return newError("Error creating file: %s", err)
			}
			defer file.Close()
			return NULL
		},
	},
	"removeFile": { //@audit-info removeFile
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return newError("'removeFile' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `removeFile` must be STRING, got %s", args[0].Type())
			}

			str := args[0].(*object.String).Value

			err := os.Remove(str)
			if err != nil {
				return newError("error removing file: %s", err)
			}

			return NULL
		},
	},
	"writeFile": { //@audit-info writeFile
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 2 {
				return newError("'writeFile' wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `writeFile` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `writeFile` must be STRING, got %s", args[1].Type())
			}

			filestr := args[0].(*object.String).Value
			str := args[1].(*object.String).Value

			file, err := os.Create(filestr)
			if err != nil {
				return newError("error opening file: %s", err)
			}
			defer file.Close()

			_, err = file.WriteString(str)
			if err != nil {
				return newError("error writing to file: %s", err)
			}

			return NULL
		},
	},
	"readFile": { //@audit-info readFile
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return newError("'readFile' wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument `readFile` must be STRING, got %s", args[0].Type())
			}

			str := args[0].(*object.String).Value

			data, err := os.ReadFile(str)
			if err != nil {
				return newError("error reading file: %s", err)
			}

			return &object.String{Value: string(data)}
		},
	},
	"reverse": { //@audit-info reverse
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'reverse' wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `reverse` must be ARRAY, got %s",
					args[0].Type())
			}
			arr := args[0].(*object.Array).Elements

			n := len(arr)
			for i := 0; i < n/2; i++ {
				arr[i], arr[n-i-1] = arr[n-i-1], arr[i]
			}

			return &object.Array{Elements: arr}
		},
	},
	"exit": { //@audit-info exit
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return newError("'exit' wrong number of arguments. got=%d, want=0",
					len(args))
			}
			os.Exit(0)
			return NULL
		},
	},
	"sleep": { //@audit-info sleep
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("'sleep' wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.INTEGER_OBJ {
				return newError("argument to `sleep` must be INTEGER, got %s",
					args[0].Type())
			}
			n := args[0].(*object.Integer).Value * 1000000
			time.Sleep(time.Duration(n))

			return NULL
		},
	},
	"keyClicked": { //@audit-info keyClicked
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return newError("'keyClicked' wrong number of arguments. got=%d, want=0",
					len(args))
			}

			const (
				ENABLE_ECHO_INPUT             = 0x0004
				ENABLE_LINE_INPUT             = 0x0002
				ENABLE_VIRTUAL_TERMINAL_INPUT = 0x0200
				STD_INPUT_HANDLE              = syscall.STD_INPUT_HANDLE
			)
			modkernel32 := syscall.NewLazyDLL("kernel32.dll")
			procGetConsoleMode := modkernel32.NewProc("GetConsoleMode")
			procSetConsoleMode := modkernel32.NewProc("SetConsoleMode")
			procReadConsole := modkernel32.NewProc("ReadConsoleW")

			h := syscall.Handle(os.Stdin.Fd())

			var oldMode uint32
			r, _, e := procGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&oldMode)))
			if r == 0 {
				return newError("getting console mode: %s", e)
			}
			newMode := oldMode &^ (ENABLE_LINE_INPUT | ENABLE_ECHO_INPUT)
			r, _, e = procSetConsoleMode.Call(uintptr(h), uintptr(newMode))
			if r == 0 {
				return newError("setting console mode: %s", e)
			}

			var buffer [1]byte
			var read uint32
			r, _, _ = procReadConsole.Call(
				uintptr(h),
				uintptr(unsafe.Pointer(&buffer[0])),
				uintptr(len(buffer)),
				uintptr(unsafe.Pointer(&read)),
				0,
			)
			procSetConsoleMode.Call(uintptr(h), uintptr(oldMode))
			if r == 0 {
				procSetConsoleMode.Call(uintptr(h), uintptr(oldMode))
				return NULL
				//return newError("reading from console: %s", e)
			}

			if read > 0 {
				char := buffer[0]
				//syscall.CloseHandle(h)
				procSetConsoleMode.Call(uintptr(h), uintptr(oldMode))
				return &object.String{Value: string(char)}
			}

			procSetConsoleMode.Call(uintptr(h), uintptr(oldMode))
			return NULL
		},
	},
	"cursorPos": { //@audit-info cursorPos
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return newError("'cursorPos' wrong number of arguments. got=%d, want=0",
					len(args))
			}

			var pt struct{ X, Y int32 }
			syscall.NewLazyDLL("user32.dll").NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))

			var newArr []object.Object
			newArr = append(newArr, &object.Integer{Value: int64(pt.X)})
			newArr = append(newArr, &object.Integer{Value: int64(pt.Y)})

			return &object.Array{Elements: newArr}
		},
	},
	"clearConsole": { //@audit-info clearConsole
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return newError("'clearConsole' wrong number of arguments. got=%d, want=0",
					len(args))
			}
			fmt.Print("\033[H\033[2J")
			return NULL
		},
	},
	"overplace": { //@audit-info overplace
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'overplace' wrong number of arguments. got=%d, want=3", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `overplace` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `overplace` must be STRING, got %s", args[1].Type())
			}
			if args[2].Type() != object.INTEGER_OBJ {
				return newError("argument[2] `overplace` must be INTEGER, got %s", args[2].Type())
			}

			str := args[0].(*object.String).Value
			insert := args[1].(*object.String).Value
			index := args[2].(*object.Integer).Value
			//length := len(str)

			if index+int64(len(insert)) <= int64(len(str)) && len(insert) <= len(str) {
				finalStr := str[:index] + insert + str[index+int64(len(insert)):]
				return &object.String{Value: finalStr}
			}
			return NULL
			/*if index < int64(length) && index > -1 {
			return &object.String{Value: string([]rune(str)[index])}
			}*/

		},
	},
	"arrEdit": { //@audit-info arrEdit
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'arrEdit' wrong number of arguments. got=%d, want=3",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument[0] `arrEdit` must be ARRAY, got %s", args[0].Type())
			}
			if args[1].Type() != object.INTEGER_OBJ {
				return newError("argument[1] `arrEdit` must be INTEGER, got %s", args[1].Type())
			}
			if args[2].Type() == object.ERROR_OBJ || args[0].Type() == object.NULL_OBJ {
				return newError("argument[2] `arrEdit` must be a valid OBJECT, got %s", args[2].Type())
			}

			arr := args[0].(*object.Array).Elements
			index := args[1].(*object.Integer).Value
			if index+1 > int64(len(arr)) || index < 0 {
				return newError("'arrEdit' argument[1] index out of range")
			}
			elem := args[2]
			var newArr []object.Object = arr
			newArr[index] = elem

			return &object.Array{Elements: newArr}
		},
	},
	"indexOf": { //@audit-info indexOf
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'indexOf' wrong number of arguments. got=%d, want=2",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument[0] `arrEdit` must be ARRAY, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `arrEdit` must be STRING, got %s", args[1].Type())
			}

			arr := args[0].(*object.Array).Elements
			findme := args[1].(*object.String).Value

			for i, ob := range arr {
				if ob.(*object.String).Value == findme {
					return &object.Integer{Value: int64(i)}
				}

			}

			return NULL
		},
	},
	"timeNow": { //@audit-info timeNow
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return newError("'timeNow' wrong number of arguments. got=%d, want=0",
					len(args))
			}

			tm := time.Now().Nanosecond()

			return &object.Float{Value: float64(tm)}
		},
	},
	"debugVar": { //@audit-info debugVar
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 0 {
				return newError("'debugVar' wrong number of arguments. got=%d, want=0",
					len(args))
			}

			store := curEnv.GetAll()
			for key, value := range store {
				valType := value.Type()
				valValue := value.Inspect()
				if valType == "FUNCTION" {
					valValue = "fn"
				}
				fmt.Printf("\n┌──────────┤\033[0;33m%s\033[0m│\n│\033[0;34m%s:\033[0m\n│%s\n└──────────", key, valType, valValue)
			}
			os.Exit(0)
			return NULL
		},
	},
	"pushFirst": { //@audit-info pushFirst
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("'pushFirst' wrong number of arguments. got=%d, want=2",
					len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `pushFirst` must be ARRAY, got %s",
					args[0].Type())
			}
			arr := args[0].(*object.Array).Elements
			elem := args[1]

			var newArr []object.Object = arr
			newArr = append([]object.Object{elem}, newArr...)

			return &object.Array{Elements: newArr}
		},
	},
	"execute": { //@audit-info execute
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("`execute` wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to `execute` must be STRING, got %s",
					args[0].Type())
			}

			return NULL
		},
	},
	"isLetter": { //@audit-info isLetter
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("`isLetter` wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to `isLetter` must be STRING, got %s",
					args[0].Type())
			}

			val := args[0].(*object.String).Value
			if len(val) == 0 {
				return &object.Boolean{Value: false}
			}

			for _, ch := range val {
				if !('a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z') {
					return &object.Boolean{Value: false}
				}
			}

			return &object.Boolean{Value: true}
		},
	},
	"isNum": { //@audit-info isNum
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("`isNum` wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to `isNum` must be STRING, got %s",
					args[0].Type())
			}

			val := args[0].(*object.String).Value
			if len(val) == 0 {
				return &object.Boolean{Value: false}
			}

			for _, ch := range val {
				if !('0' <= ch && ch <= '9') {
					return &object.Boolean{Value: false}
				}
			}

			return &object.Boolean{Value: true}

		},
	},
	"contains": { //@audit-info contains
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("`contains` wrong number of arguments. got=%d, want=1",
					len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] to `contains` must be STRING, got %s",
					args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] to `contains` must be STRING, got %s",
					args[1].Type())
			}
			value := args[0].(*object.String).Value
			contained := args[1].(*object.String).Value

			StartrightCount := len(contained)
			if StartrightCount == 0 {
				return newError("argument[1] `contains` is an empty STRING")
			}

			rightCount := StartrightCount
			curT := rune(contained[0])
			for _, c := range value {
				if rightCount <= 0 {
					return &object.Boolean{Value: true}
				}
				if c == curT {
					rightCount--
					if rightCount > 0 {
						curT = rune(contained[StartrightCount-rightCount])
					}
				} else {
					rightCount = StartrightCount
					curT = rune(contained[0])
				}
			}
			return &object.Boolean{Value: false}
		},
	},
	"hashEdit": { //@audit-info hashEdit
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("'hashEdit' wrong number of arguments. got=%d, want=3",
					len(args))
			}
			if args[0].Type() != object.HASH_OBJ {
				return newError("argument[0] `hashEdit` must be HASH, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `hashEdit` must be STRING, got %s", args[1].Type())
			}
			if args[2].Type() == object.ERROR_OBJ || args[0].Type() == object.NULL_OBJ {
				return newError("argument[2] `hashEdit` must be a valid OBJECT, got %s", args[2].Type())
			}

			hash := args[0].(*object.Hash).Pairs
			target := args[1].(*object.String)
			elem := args[2]

			var newhash map[object.HashKey]object.HashPair = hash

			newhash[target.HashKey()] = object.HashPair{Key: target, Value: elem}

			return &object.Hash{Pairs: newhash}
		},
	},
	"callDLL": { //@audit-info callDLL
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 12 {
				return newError("'callDLL' wrong number of arguments. got=%d, want=2-12", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument[0] `callDLL` must be STRING, got %s", args[0].Type())
			}
			if args[1].Type() != object.STRING_OBJ {
				return newError("argument[1] `callDLL` must be STRING, got %s", args[1].Type())
			}
			if args[2].Type() != object.STRING_OBJ {
				return newError("argument[2] `callDLL` must be STRING, got %s", args[2].Type())
			}

			dllName := args[0].(*object.String).Value
			funcName := args[1].(*object.String).Value
			funcType := args[2].(*object.String).Value

			var inputs = [10]uintptr{}
			dll := syscall.NewLazyDLL(dllName)
			if err := dll.Load(); err != nil {
				return newError("Error loading DLL: " + err.Error())
			}
			proc := dll.NewProc(funcName)
			if proc == nil {
				return newError("Error finding function: " + funcName)
			}
			// Read inputs
			for i := 3; i < len(args); i++ {
				if args[i].Type() == object.ERROR_OBJ || args[i].Type() == object.NULL_OBJ {
					return newError("argument[%d] `callDLL` must be a valid OBJECT, got %s", i, args[i].Type())
				} else {
					switch args[i].Type() {
					case object.INTEGER_OBJ:
						v := args[i].(*object.Integer)
						inputs[i-3] = uintptr(v.Value)

					case object.STRING_OBJ:
						s := args[i].(*object.String).Value
						cstr := syscall.StringBytePtr(s)
						inputs[i-3] = uintptr(unsafe.Pointer(cstr))

					}
				}
			}
			//make call
			r1, _, err := proc.Call(inputs[0], inputs[1], inputs[2], inputs[3], inputs[4], inputs[5], inputs[6], inputs[7], inputs[8], inputs[9])

			if errno, ok := err.(syscall.Errno); ok && errno != 0 && errno != 2 && errno != 127 && errno != 1400 && errno != 87 && errno != 6 {
				return newError("Error calling function: errno=%d, description=%s\n", errno, errno.Error())
			}

			//detect output
			switch funcType {
			case "int":
				var FValue int64
				maxPositive := uint(1) << (32 - 1)
				if uint(r1) > maxPositive {
					if uint(r1) >= maxPositive {
						FValue = int64(uint(r1) - (1 << 32))
					}
				} else {
					FValue = int64(r1)
				}
				return &object.Integer{Value: FValue}
			case "bool":
				if uint(r1) == 1 {
					return &object.Boolean{Value: true}
				}
				return &object.Boolean{Value: false}
			case "string":
				ptr := (*byte)(unsafe.Pointer(r1))
				var bytes []byte
				for *ptr != 0 {
					bytes = append(bytes, *ptr)
					ptr = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + 1))
				}

				input := string(bytes)
				var result string
				for i := 0; i < len(input); {
					_, size := utf8.DecodeRuneInString(input[i:])
					result += string(input[i])
					i += size
				}

				return &object.String{Value: result}
			case "void":
				return NULL

			}

			return NULL
		},
	},
}
