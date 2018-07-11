// miniMAL
// Copyright (C) 2018 Jordi Íñigo i Griera
// Licensed under MPL 2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Environment contains the scope symbols
type Environment struct {
	Scope  map[string]interface{}
	Parent *Environment
}

func functionAdd(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return json.Number(strconv.FormatInt(a+b, 10))
}

func functionSub(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return json.Number(strconv.FormatInt(a-b, 10))
}

func functionMul(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return json.Number(strconv.FormatInt(a*b, 10))
}

func functionDiv(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return json.Number(strconv.FormatInt(a/b, 10))
}

func functionEqual(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return a == b
}

func functionLT(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return a < b
}

func functionGT(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return a > b
}

func functionGE(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return a >= b
}

func functionLE(args []interface{}) interface{} {
	a, b := _arith2ints(args)
	return a <= b
}

func _arith2ints(args []interface{}) (a, b int64) {
	var err error
	a, err = args[0].(json.Number).Int64()
	if err != nil {
		panic(err)
	}
	b, err = args[1].(json.Number).Int64()
	if err != nil {
		panic(err)
	}
	return a, b
}

func _arith1int(args interface{}) (a int64) {
	var err error
	a, err = args.(json.Number).Int64()
	if err != nil {
		panic(err)
	}
	return a
}

// BaseSymbolTable returns a symbol table with predefined contents
func BaseSymbolTable() (env *Environment) {
	env = &Environment{
		Scope: map[string]interface{}{
			"+":  args2(functionAdd),
			"*":  args2(functionMul),
			"-":  args2(functionSub),
			"/":  args2(functionDiv),
			"<":  args2(functionLT),
			"<=": args2(functionLE),
			">":  args2(functionGT),
			">=": args2(functionGE),
			"=": args2(func(args []interface{}) interface{} {
				if reflect.ValueOf(args[0]).Type() != reflect.ValueOf(args[1]).Type() {
					return false
				}
				switch a := args[0].(type) {
				case json.Number:
					return functionEqual(args)
				case string:
					return strings.Compare(a, args[1].(string)) == 0
				}
				return reflect.DeepEqual(args[0], args[1])
			}),
			"list": argsVariadic(func(args []interface{}) interface{} { return args }),
			"map": args1(func(args []interface{}) interface{} {
				result := make([]interface{}, len(args)-1)
				for i, value := range args[1:] {
					f := args[0].(func([]interface{}) interface{})
					result[i] = f([]interface{}{value})
				}
				return result
			}),

			// FILESYSTEM
			"eval": args1(func(args []interface{}) interface{} {
				return EVAL(args[0], env)
			}),
			"read":  args1(functionRead),
			"slurp": args1(functionSlurp),
			"load": args1(func(args []interface{}) interface{} {
				// functionLoad reads an AST from file
				fileContents := functionSlurp(args)
				ast := functionRead([]interface{}{fileContents.(string)})
				return EVAL(ast, env)
			}),
			"str":     argsVariadic(functionStr),
			"pr-str":  argsVariadic(functionPrStr),
			"prn":     argsVariadic(functionPrn),
			"println": argsVariadic(functionPrintln),
			"print":   argsVariadic(functionPrint),
			"list?":   args1(functionListQ),
			"count":   args1(functionCount),
			"empty?":  args1(functionEmptyQ),
			"string?": args1(functionStringQ),
			"first":   args1(functionFirst),
			"last":    args1(functionLast),
			"nth":     args2(functionNth),
		},
	}
	return env
}

func functionFirst(args []interface{}) interface{} {
	switch arg0 := args[0].(type) {
	case []interface{}:
		l := len(arg0)
		if l == 0 {
			return nil
		}
		return arg0[0]
	default:
		panic(fmt.Errorf("first argument must be a list"))
	}
}

func functionLast(args []interface{}) interface{} {
	switch arg0 := args[0].(type) {
	case []interface{}:
		l := len(arg0)
		if l == 0 {
			return nil
		}
		return arg0[l-1]
	default:
		panic(fmt.Errorf("last argument must be a list"))
	}
}

func functionNth(args []interface{}) interface{} {
	switch args[1].(type) {
	case json.Number:
		n := _arith1int(args[1])
		switch arg0 := args[0].(type) {
		case []interface{}:
			lenght := int64(len(arg0))
			if lenght < n {
				return nil
			}
			return arg0[n]
		default:
			panic(fmt.Errorf("nth second argument must be a list"))
		}
	default:
		panic(fmt.Errorf("nth first argument must be a number"))
	}
}

func functionStringQ(args []interface{}) interface{} {
	_, ok := args[0].(string)
	return ok
}

func functionListQ(args []interface{}) interface{} {
	_, ok := args[0].([]interface{})
	return ok
}

func functionCount(args []interface{}) interface{} {
	elements, ok := args[0].([]interface{})
	if !ok {
		panic(fmt.Errorf("Not a list"))
	}
	return json.Number(strconv.Itoa(len(elements)))
}

func functionEmptyQ(args []interface{}) interface{} {
	elements, ok := args[0].([]interface{})
	if !ok {
		panic(fmt.Errorf("Not a list"))
	}
	return len(elements) == 0
}

func functionStr(args []interface{}) interface{} {
	strs := ""
	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			strs += arg
		case []interface{}:
			strs += functionStr(arg).(string)
		default:
			strs += JSON(arg)
		}
	}
	return strs
}

func functionPrStr(args []interface{}) interface{} {
	strs := []string{}
	for _, arg := range args {
		strs = append(strs, JSON(arg))
	}
	return strings.Join(strs, " ")
}

func functionPrn(args []interface{}) interface{} {
	fmt.Println(functionPrStr(args))
	return nil
}

func functionPrintln(args []interface{}) interface{} {
	fmt.Println(functionStr(args))
	return nil
}

func functionPrint(args []interface{}) interface{} {
	fmt.Print(functionStr(args))
	return nil
}

// functionRead reads a string
func functionRead(args []interface{}) interface{} {
	return READ(args[0].(string))
}

// functionSlurp reads a file
func functionSlurp(args []interface{}) interface{} {
	switch fileName := args[0].(type) {
	case string:
		contents, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}
		return string(contents)
	default:
		panic(fmt.Errorf("slurp requires a filename"))
	}
}

func args1(f func(args []interface{}) interface{}) func(args []interface{}) interface{} {
	return func(args []interface{}) interface{} {
		if len(args) != 1 {
			panic(fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args)))
		}
		return f(args)
	}
}

func args2(f func(args []interface{}) interface{}) func(args []interface{}) interface{} {
	return func(args []interface{}) interface{} {
		if len(args) != 2 {
			panic(fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args)))
		}
		return f(args)
	}
}

func argsVariadic(f func(args []interface{}) interface{}) func(args []interface{}) interface{} {
	return func(args []interface{}) interface{} {
		return f(args)
	}
}

// NewSymbolTable creates a copy of an environtment table
func NewSymbolTable(parent *Environment) *Environment {
	return &Environment{
		Scope:  map[string]interface{}{},
		Parent: parent,
	}
}

// Get returns the value of a symbol
func (e *Environment) Get(index string) interface{} {
	value, ok := e.Scope[index]
	if !ok {
		if e.Parent == nil {
			panic(fmt.Errorf("Symbol %q undefined", index))
		}
		return e.Parent.Get(index)
	}
	return value
}

// Set defines a new symbol
func (e *Environment) Set(index string, value interface{}) interface{} {
	e.Scope[index] = value
	return value
}

// READ parses a JSON encoded string and unmarshals it to an Atom
func READ(str string) (ast interface{}) {
	switch str {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}

	switch str[0] {
	case '{':
		ast = map[string]interface{}{}
	case '[':
		ast = []interface{}{}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		var number json.Number
		ast = number
	case '"':
		ast = ""
	default:
		panic(fmt.Errorf("Cannot unmarshal: %s", str))
	}
	dec := json.NewDecoder(strings.NewReader(str))
	dec.UseNumber()

	if err := dec.Decode(&ast); err != nil {
		panic(err)
	}
	return ast
}

func evalAST(ast interface{}, env *Environment) interface{} {
	switch ast := ast.(type) {
	case []interface{}:
		outAST := make([]interface{}, len(ast))
		for i, atom := range ast {
			outAST[i] = EVAL(atom, env)
		}
		return outAST
	case string:
		return env.Get(ast)
	default:
		return ast
	}
}

func envBind(ast interface{}, env *Environment, expressions []interface{}) *Environment {
	switch ast := ast.(type) {
	case []interface{}:
		newEnv := NewSymbolTable(env)
		for i, atom := range ast {
			switch atom := atom.(type) {
			default:
				panic(fmt.Errorf("Variable identifier must be a string (was %T)", atom))
			case string:
				if atom == "&" {
					if i+1 == len(ast) {
						panic(fmt.Errorf("binding list cannot end with &"))
					}
					newEnv.Set(ast[i+1].(string), expressions[i:])
					return newEnv
				}
				newEnv.Set(atom, expressions[i])
			}
		}
		return newEnv
	default:
		panic(fmt.Errorf("Binding must receive an array"))
	}
}

type tcoFN struct {
	f          func(args []interface{}) interface{}
	bodyAST    interface{}
	env        *Environment
	argSpecAST interface{}
}

// EVAL returns an atom after evaluating an atom entry
func EVAL(ast interface{}, env *Environment) interface{} {
	for {
		// fmt.Printf("(ง'̀-'́)ง %[1]T %[1]s\n", ast)
		switch typedAST := ast.(type) {
		case []interface{}:
			switch first := typedAST[0].(type) {
			case string:
				switch first {

				// apply
				case "def":
					identifier, ok := typedAST[1].(string)
					if !ok {
						panic(fmt.Errorf("Second argument in def %q must be a string name", typedAST[1]))
					}
					value := EVAL(typedAST[2], env)
					env.Set(identifier, value)
					return value
				case "`": // quote
					return typedAST[1]
				case "fn":
					if len(typedAST) != 3 {
						panic(fmt.Errorf("fn need 2 arguments (found %d)", len(typedAST)))
					}
					return tcoFN{
						f: func(args []interface{}) interface{} {
							newEnv := envBind(typedAST[1], env, args)
							return EVAL(typedAST[2], newEnv)
						},
						bodyAST:    typedAST[2],
						env:        env,
						argSpecAST: typedAST[1],
					}

				// TCO
				case "let":
					newEnv := NewSymbolTable(env)
					variables, ok := typedAST[1].([]interface{})
					if !ok {
						panic(fmt.Errorf("Second argument in let must be a list"))
					}
					if len(variables)%2 != 0 {
						panic(fmt.Errorf("Second argument in let must be a list of pairs of name value"))
					}
					for i := range variables {
						if i%2 != 0 {
							continue
						}
						value := EVAL(variables[i+1], newEnv)
						newEnv.Set(variables[i].(string), value)
					}
					env = newEnv
					ast = typedAST[2].([]interface{})
					goto contTCO
				case "if":
					evaledCondition := EVAL(typedAST[1], env)
					var ifCondition bool
					switch evaledCondition := evaledCondition.(type) {
					case bool:
						ifCondition = evaledCondition
					case json.Number:
						ifCondition = functionEqual([]interface{}{evaledCondition, 0}).(bool)
					case nil:
						ifCondition = false
					case []interface{}:
						ifCondition = len(evaledCondition) > 0
					case string:
						ifCondition = evaledCondition != ""
					default:
						panic(fmt.Errorf("if requires a quasi boolean condition but got %T", evaledCondition))
					}

					if ifCondition {
						ast = typedAST[2]
					} else {
						ast = typedAST[3]
					}
					goto contTCO
				case "do":
					if len(typedAST) > 2 {
						evalAST(typedAST[1:len(typedAST)-1], env)
					}
					ast = typedAST[len(typedAST)-1]
					goto contTCO
				}
			}

			// default cases for both switches
			// -> fnCall(ast, env)
			elements := evalAST(typedAST, env)

			switch elements := elements.(type) {
			case []interface{}:
				f := elements[0]
				switch f := f.(type) {
				case func([]interface{}) interface{}:
					result := f(elements[1:])
					return result
				case tcoFN:
					ast = f.bodyAST
					env = envBind(f.argSpecAST, f.env, elements[1:])
					goto contTCO
				default:
					panic(fmt.Errorf("Non callable atom %T", f))
				}
			default:
				panic(fmt.Errorf("?? BOGUS %T", elements))
			}
		default:
			return evalAST(ast, env)
		}
	contTCO:
		// fmt.Printf("        %[1]T %[1]s\n", ast)
	}
}

// JSON returns the atom JSON sencoded
func JSON(ast interface{}) string {
	b, err := json.Marshal(ast)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func main() {
	// symbolTable := BaseSymbolTable()
	// line := `["if", [">", ["count", ["list", 1, 2, 3]], 3], ["` + "`" + `", "yes"], ["` + "`" + `", "no"]]`
	// result := JSON(EVAL(READ(line), symbolTable))
	// fmt.Println(result)
	// os.Exit(0)

	if len(os.Args) >= 2 {
		symbolTable := BaseSymbolTable()
		symbolTable.Set("ARGS", os.Args[2:])

		args := make([]interface{}, len(os.Args))
		for i := range os.Args {
			args[i] = os.Args[i]
		}
		EVAL([]interface{}{"load", []interface{}{"`", args[1]}}, symbolTable)
	} else {
		symbolTable := BaseSymbolTable()
		symbolTable.Set("ARGS", os.Args[1:]) // inneeded

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("> ")
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				os.Exit(0)
			}

			line = strings.Trim(line, " \t\n")
			if len(line) == 0 {
				continue
			}

			fmt.Println(JSON(EVAL(READ(line), symbolTable)))
		}
	}
}
