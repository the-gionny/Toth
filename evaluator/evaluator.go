package evaluator

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"io"

	"os"
	"path/filepath"
	"toth/ast"
	"toth/lexer"
	"toth/object"
	"toth/parser"
)

var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NULL  = &object.Null{}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	//fmt.Println(len(node.String()) / 4)
	switch node := node.(type) {

	case *ast.HashLiteral:
		return evalHashLiteral(node, env)

	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}

		//println("index1", node.Left.String(), node.Index.String())
		return evalIndexExpression(left, index)

	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.CallExpression:
		function := Eval(node.Function, env)

		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)

		if function.Type() == object.FUNCTION_OBJ {
			params := function.(*object.Function).Parameters
			if len(params) != len(args) {
				return newError("'%s' number of paramenter not correct. expected=%d", node.Function.String(), len(params)-len(args))
			}

		}

		if len(args) == 1 {
			if args[0] == nil {
				return newError("the function don't have a return value")
			}
			if isError(args[0]) {
				return args[0]
			}
		}

		return applyFunction(function, args)
	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: env, Body: body}
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.VarStatement:

		val := Eval(node.Value, env)
		//println(val.Type())

		//ban builtin functions
		if builtins[node.Name.Value] != nil {
			return newError("Attempt to redefine built-in function '%s'.", node.Name.Value)
		}
		if isError(val) {
			return val
		}
		if node.Global {
			env.GlobalSet(node.Name.Value, val)
		} else {

			switch node.Operator {
			case "+=":
				Nval, ok := env.Get(node.Name.Value)
				if !ok {
					return newError("variable '%s' is not defined. Please declare and initialize '%s' before using it.", node.Name, node.Name)
				}
				env.Set(node.Name.Value, evalInfixExpression("+", Nval, val))
			case "-=":
				Nval, ok := env.Get(node.Name.Value)
				if !ok {
					return newError("variable '%s' is not defined. Please declare and initialize '%s' before using it.", node.Name, node.Name)
				}
				env.Set(node.Name.Value, evalInfixExpression("-", Nval, val))
			default:
				env.Set(node.Name.Value, val)
			}
		}
	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.IfExpression:
		return evalIfExpression(node, env)
	case *ast.WhileExpression:
		return evalWhileExpression(node, env)
	case *ast.ForExpression:
		return evalForExpression(node, env)
	case *ast.UsingExpression:
		return evalUsingExpression(node, env)

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.InfixExpression:

		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}

		return evalInfixExpression(node.Operator, left, right)
	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	}
	return nil
}

func evalHashLiteral(
	node *ast.HashLiteral,
	env *object.Environment,
) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("unusable as hash key: %s", key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}
		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}
	return &object.Hash{Pairs: pairs}
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return newError("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}
	return pair.Value
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)
	//println("found index: ", idx)

	if idx < 0 || idx > max {
		return NULL
	}
	return arrayObject.Elements[idx]
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}

}

func extendFunctionEnv(
	fn *object.Function,
	args []object.Object,
) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}
	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}
	return result
}

func evalIdentifier(
	node *ast.Identifier,
	env *object.Environment,
) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if builtin, ok := builtins[node.Value]; ok {

		//println(node.Token.Line)
		curEnv = *env
		//println("imported a builtin ", node.Value)
		return builtin
	}
	return newError("Line: %d - identifier not found: %s", node.Token.Line, node.Value)
}

func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalWhileExpression(we *ast.WhileExpression, env *object.Environment) object.Object {
	if isTruthy(Eval(we.Condition, env)) {
		con := Eval(we.Consequence, env)
		if isError(con) {
			return con
		}
	}

	condition := Eval(we.Condition, env)
	if isError(condition) {
		return condition
	}
	for isTruthy(Eval(we.Condition, env)) {
		Eval(we.Consequence, env)
	}
	return NULL
}

// for-loop
func evalForExpression(fe *ast.ForExpression, env *object.Environment) object.Object {

	VarValue := Eval(fe.Var.Value, env)
	env.Set(fe.Var.Name.Value, VarValue)

	if VarValue.Type() != object.INTEGER_OBJ {
		return newError("the counter can only be an INTEGER. got=%s", VarValue.Type())
	}

	ValueTwo := Eval(fe.Iteration, env)

	if ValueTwo.Type() != object.INTEGER_OBJ {
		return newError("the iteration can only be an INTEGER. got=%s", ValueTwo.Type())
	}

	condition := Eval(fe.Condition, env)

	if isError(condition) {
		return condition
	}

	/*
		if !strings.Contains(fe.Condition.String(), fe.Var.Name.Value) {
			return newError("the condition must contain the counter. got=%s", fe.Condition.String())
		}*/

	for isTruthy(Eval(fe.Condition, env)) {
		con := Eval(fe.Consequence, env)
		if isError(con) {
			return con
		}

		VarValueGet, _ := env.Get(fe.Var.Name.Value)
		if VarValueGet.Type() != object.INTEGER_OBJ {
			return newError("the counter can only be an INTEGER. got=%s", VarValueGet.Type())
		}

		NewValue := +ValueTwo.(*object.Integer).Value + VarValueGet.(*object.Integer).Value
		env.Set(fe.Var.Name.Value, &object.Integer{Value: NewValue})

	}
	env.Reset(fe.Var.Name.Value)
	return NULL
}

// Executes th files
const (
	colorReset = "\033[0m"
	colorRed   = "\033[0;31m"
)

func evalUsingExpression(ue *ast.UsingExpression, env *object.Environment) object.Object {
	tg := Eval(ue.Target, env)
	if isError(tg) {
		return tg
	}
	if tg.Type() != object.STRING_OBJ {
		return newError("can only import from name STRING. got=%s", tg.Type())
	}

	path, _ := os.Getwd()
	println(path)

	modName := ue.Target.String()
	targetPath := modName + ".th"
	absolutePath, err := filepath.Abs(targetPath)
	if err != nil {
		return newError("cannot find module named: '%s' %s", modName, absolutePath)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		targetPath = modName + ".go"
		absolutePath, _ = filepath.Abs(targetPath)
		_, err = os.ReadFile(targetPath)
		if err != nil {
			return newError("cannot find module named: '%s' %s", modName, absolutePath+"/.th")
		}
		//@audit import GO module
		fset := gotoken.NewFileSet()
		node, _ := goparser.ParseFile(fset, absolutePath, nil, goparser.ParseComments)
		/*node, err := goparser.ParseFile(fset, absolutePath, nil, goparser.ParseComments)
		if err != nil {
			return newError("Module error %s: %s", modName, err)
		}*/
		goast.Inspect(node, func(n goast.Node) bool {
			if fn, ok := n.(*goast.FuncDecl); ok {
				fmt.Println("Function found:", fn.Name.Name)
				for _, param := range fn.Type.Params.List {
					for _, name := range param.Names {
						fmt.Printf("Arg: %s\n", name)
					}
				}
			}
			return true
		})

	} else {
		//@audit import TH module
		contentWithoutNewlines := string(data)
		var out io.Writer = os.Stdout
		fmt.Fprint(out)

		line := contentWithoutNewlines
		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()

		if len(p.Errors()) != 0 {
			io.WriteString(out, colorRed+"---"+modName+"---\n"+colorReset)
			io.WriteString(out, colorRed+" Toth: the program ran into some problem!\n"+colorReset)
			for _, msg := range p.Errors() {
				io.WriteString(out, colorRed+"\t"+msg+"\n"+colorReset)
			}
			return NULL
		}

		evaluated := Eval(program, env)
		if evaluated != nil {
			if evaluated.Type() == object.ERROR_OBJ {
				io.WriteString(out, evaluated.Inspect())
			}
		}
	}

	return NULL
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return false
	}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result

		}

	}
	return result
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE

	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER_OBJ && right.Type() != object.FLOAT_OBJ {
		return newError("unknown operator: -%s", right.Type())
	}
	switch right.Type() {
	case object.INTEGER_OBJ:
		value := right.(*object.Integer).Value
		return &object.Integer{Value: -value}
	case object.FLOAT_OBJ:
		value := right.(*object.Float).Value
		return &object.Float{Value: -value}
	}
	return NULL
}

func evalInfixExpression(operator string, left, right object.Object,
) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)

	case left.Type() == object.INTEGER_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalIntFloatInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalFloatIntInfixExpression(operator, left, right)

	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)

	case left.Type() == object.STRING_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalStringIntegerInfixExpression(operator, left, right)

	case left.Type() == object.STRING_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalStringFloatInfixExpression(operator, left, right)

	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case operator == "&&":
		leftVal := left.(*object.Boolean).Value
		rightVal := right.(*object.Boolean).Value
		return nativeBoolToBooleanObject(leftVal && rightVal)
	case operator == "||":
		leftVal := left.(*object.Boolean).Value
		rightVal := right.(*object.Boolean).Value
		return nativeBoolToBooleanObject(leftVal || rightVal)
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}
func evalStringIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.Integer).Value
	switch operator {
	case "*":
		if rightVal < 0 {
			return newError("cannot multiply string: repeat count must be non-negative '%d'!", rightVal)
		}
		result := ""
		for i := 0; i < int(rightVal); i++ {
			result += leftVal
		}
		return &object.String{Value: result}
	case "+":
		return &object.String{Value: leftVal + right.(*object.Integer).Inspect()}
	}
	return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
}
func evalStringFloatInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	switch operator {
	case "+":
		return &object.String{Value: leftVal + right.(*object.Float).Inspect()}
	}
	return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	}
	return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())

}

func PowInteger(base, e int64) int64 {
	result := int64(1)
	for i := int64(0); i < e; i++ {
		result *= base
	}
	return result
}
func PowFloat(base, e float64) float64 {
	result := float64(1)
	for i := float64(0); i < e; i++ {
		result *= base
	}
	return result
}

func evalFloatIntInfixExpression(operator string, left, right object.Object,
) object.Object {
	leftVal := left.(*object.Float).Value
	rightVal := float64(right.(*object.Integer).Value)

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "%":
		return &object.Float{Value: float64(int(leftVal) % int(rightVal))}
	case "^":
		return &object.Float{Value: PowFloat(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}
func evalIntFloatInfixExpression(operator string, left, right object.Object,
) object.Object {
	leftVal := float64(left.(*object.Integer).Value)
	rightVal := right.(*object.Float).Value

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "%":
		return &object.Float{Value: float64(int(leftVal) % int(rightVal))}
	case "^":
		return &object.Float{Value: PowFloat(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}
func evalIntegerInfixExpression(operator string, left, right object.Object,
) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "%":
		return &object.Integer{Value: leftVal % rightVal}
	case "^":
		return &object.Integer{Value: PowInteger(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object,
) object.Object {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "%":
		return &object.Float{Value: float64(int(leftVal) % int(rightVal))}
	case "^":
		return &object.Float{Value: PowFloat(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func newError(format string, a ...interface{}) *object.Error {

	//format = node.Token.Line
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}
