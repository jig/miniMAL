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
	"log"
	"os"
	"reflect"
	"strings"
)

type Environment struct {
	Scope  map[string]interface{}
	Parent *Environment
}

func (env *Environment) Dump() string {
	str := ""
	if env.Parent != nil {
		str = env.Parent.Dump()
	}
	for k, _ := range env.Scope {
		str += k + " "
	}
	return str + "\n"
}

// BaseSymbolTable returns a symbol table with predefined contents
func BaseSymbolTable() (env *Environment) {
	env = &Environment{
		Scope: map[string]interface{}{
			"+": argsVariadic(func(args []interface{}) (interface{}, error) {
				result := float64(0)
				for _, v := range args {
					result += v.(float64)
				}
				return result, nil
			}),
			"*": argsVariadic(func(args []interface{}) (interface{}, error) {
				result := float64(1)
				for _, v := range args {
					result *= v.(float64)
				}
				return result, nil
			}),
			"-": args2(func(args []interface{}) (interface{}, error) { return args[0].(float64) - args[1].(float64), nil }),
			"/": args2(func(args []interface{}) (interface{}, error) { return args[0].(float64) / args[1].(float64), nil }),
			"=": args2(func(args []interface{}) (interface{}, error) {
				if reflect.ValueOf(args[0]).Type() != reflect.ValueOf(args[1]).Type() {
					return false, nil
				}
				switch a := args[0].(type) {
				case float64:
					return a == args[1].(float64), nil
				case string:
					return strings.Compare(a, args[1].(string)) == 0, nil
				}
				// FIXME: this is not efficient, used only when an array is to be compared
				return reflect.DeepEqual(args[0], args[1]), nil
			}),
			"<": args2(func(args []interface{}) (interface{}, error) {
				switch a := args[0].(type) {
				case float64:
					return a < args[1].(float64), nil
				case string:
					return strings.Compare(a, args[1].(string)) == -1, nil
				default:
					return nil, fmt.Errorf("Cannot compare type %T", a)
				}
			}),
			"<=": args2(func(args []interface{}) (interface{}, error) {
				switch a := args[0].(type) {
				case float64:
					return a <= args[1].(float64), nil
				case string:
					return strings.Compare(a, args[1].(string)) != 1, nil
				default:
					return nil, fmt.Errorf("Cannot compare type %T", a)
				}
			}),
			">": args2(func(args []interface{}) (interface{}, error) {
				switch a := args[0].(type) {
				case float64:
					return a > args[1].(float64), nil
				case string:
					return strings.Compare(a, args[1].(string)) == 1, nil
				default:
					return nil, fmt.Errorf("Cannot compare type %T", a)
				}
			}),
			">=": args2(func(args []interface{}) (interface{}, error) {
				switch a := args[0].(type) {
				case float64:
					return a >= args[1].(float64), nil
				case string:
					return strings.Compare(a, args[1].(string)) != -1, nil
				default:
					return nil, fmt.Errorf("Cannot compare type %T", a)
				}
			}),
			"list": argsVariadic(func(args []interface{}) (interface{}, error) { return args, nil }),
			"map": args1(func(args []interface{}) (interface{}, error) {
				result := make([]interface{}, len(args)-1)
				for i, value := range args[1:] {
					var err error
					f := args[0].(func([]interface{}) (interface{}, error))
					result[i], err = f([]interface{}{value})
					if err != nil {
						return nil, err
					}
				}
				return result, nil
			}),

			// FILESYSTEM
			"eval": args1(func(args []interface{}) (interface{}, error) {
				return EVAL(args[0], env)
			}),
			"read":  args1(functionRead),
			"slurp": args1(functionSlurp),
			"load": args1(func(args []interface{}) (interface{}, error) {
				// functionLoad reads an AST from file
				fileContents, err := functionSlurp(args)
				if err != nil {
					return nil, err
				}

				ast, err := functionRead([]interface{}{fileContents.(string)})
				if err != nil {
					return nil, err
				}
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

func functionFirst(args []interface{}) (interface{}, error) {
	switch arg0 := args[0].(type) {
	case []interface{}:
		l := len(arg0)
		if l == 0 {
			return nil, nil
		}
		return arg0[0], nil
	default:
		return nil, fmt.Errorf("first argument must be a list")
	}
}

func functionLast(args []interface{}) (interface{}, error) {
	switch arg0 := args[0].(type) {
	case []interface{}:
		l := len(arg0)
		if l == 0 {
			return nil, nil
		}
		return arg0[l-1], nil
	default:
		return nil, fmt.Errorf("last argument must be a list")
	}
}

func functionNth(args []interface{}) (interface{}, error) {
	switch n := args[1].(type) {
	case float64:
		switch arg0 := args[0].(type) {
		case []interface{}:
			l := len(arg0)
			if l < int(n) {
				return nil, nil
			}
			return arg0[int(n)], nil
		default:
			return nil, fmt.Errorf("nth second argument must be a list")
		}
	default:
		return nil, fmt.Errorf("nth first argument must be a number")
	}
}

func functionStringQ(args []interface{}) (interface{}, error) {
	_, ok := args[0].(string)
	return ok, nil
}

func functionListQ(args []interface{}) (interface{}, error) {
	_, ok := args[0].([]interface{})
	return ok, nil
}

func functionCount(args []interface{}) (interface{}, error) {
	elements, ok := args[0].([]interface{})
	if !ok {
		return nil, fmt.Errorf("Not a list")
	}
	return float64(len(elements)), nil
}

func functionEmptyQ(args []interface{}) (interface{}, error) {
	count, err := functionCount(args)
	if err != nil {
		return nil, err
	}
	return count.(float64) == 0, nil
}

func functionStr(args []interface{}) (interface{}, error) {
	strs := ""
	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			strs += arg
		case []interface{}:
			a, err := functionStr(arg)
			if err != nil {
				return nil, err
			}
			strs += a.(string)
		default:
			bArg, err := json.Marshal(arg)
			if err != nil {
				return nil, err
			}
			strs += string(bArg)
		}
	}
	return strs, nil
}

func functionPrStr(args []interface{}) (interface{}, error) {
	strs := []string{}
	for _, arg := range args {
		bArg, err := json.Marshal(arg)
		if err != nil {
			return nil, err
		}
		strs = append(strs, string(bArg))
	}
	return strings.Join(strs, " "), nil
}

func functionPrn(args []interface{}) (interface{}, error) {
	strs := []string{}
	for _, arg := range args {
		b, err := json.Marshal(arg)
		if err != nil {
			return nil, err
		}
		strs = append(strs, string(b))
	}
	fmt.Println(strings.Join(strs, " "))
	return nil, nil
}

func functionPrintln(args []interface{}) (interface{}, error) {
	str, err := functionStr(args)
	if err != nil {
		return nil, err
	}
	fmt.Println(str)
	return nil, nil
}

func functionPrint(args []interface{}) (interface{}, error) {
	str, err := functionStr(args)
	if err != nil {
		return nil, err
	}
	fmt.Print(str)
	return nil, nil
}

// functionRead reads a string
func functionRead(args []interface{}) (interface{}, error) {
	switch arg := args[0].(type) {
	case string:
		return READ([]byte(arg))
	default:
		return nil, fmt.Errorf("read argument must be a string but was %T", args[0])
	}
}

// functionSlurp reads a file
func functionSlurp(args []interface{}) (interface{}, error) {
	switch fileName := args[0].(type) {
	case string:
		contents, err := ioutil.ReadFile(fileName)
		if err != nil {
			return nil, err
		}
		return string(contents), nil
	default:
		return nil, fmt.Errorf("slurp requires a filename")
	}
}

func args1(f func(args []interface{}) (interface{}, error)) func(args []interface{}) (interface{}, error) {
	return func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args)
	}
}

func args2(f func(args []interface{}) (interface{}, error)) func(args []interface{}) (interface{}, error) {
	return func(args []interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args)
	}
}

func argsVariadic(f func(args []interface{}) (interface{}, error)) func(args []interface{}) (interface{}, error) {
	return func(args []interface{}) (interface{}, error) {
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
func (e *Environment) Get(index string) (interface{}, error) {
	value, ok := e.Scope[index]
	if !ok {
		if e.Parent == nil {
			return nil, fmt.Errorf("Symbol %q undefined", index)
		}
		return e.Parent.Get(index)
	}
	return value, nil
}

// Set defines a new symbol
func (e *Environment) Set(index string, value interface{}) (interface{}, error) {
	e.Scope[index] = value
	return value, nil
}

// READ parses a JSON encoded string and unmarshals it to an Atom
func READ(b []byte) (ast interface{}, err error) {
	switch string(b) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}

	switch b[0] {
	case '{':
		ast = map[string]interface{}{}
	case '[':
		ast = []interface{}{}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		ast = float64(0) // FIXME: json decoding by default decodes numbers to float64. Change default behaviour to int64
	case '"':
		ast = ""
	default:
		err = fmt.Errorf("Cannot unmarshal: %s", string(b))
	}
	err = json.Unmarshal(b, &ast)
	if err != nil {
		return nil, err
	}
	return ast, nil
}

func evalAST(ast interface{}, env *Environment) (interface{}, error) {
	switch ast := ast.(type) {
	case []interface{}:
		outAST := make([]interface{}, len(ast))
		for i, atom := range ast {
			var err error
			outAST[i], err = EVAL(atom, env)
			if err != nil {
				return nil, err
			}
		}
		return outAST, nil
	case string:
		return env.Get(ast)
	default:
		return ast, nil
	}
}

func envBind(ast interface{}, env *Environment, expressions []interface{}) (*Environment, error) {
	switch ast := ast.(type) {
	case []interface{}:
		newEnv := NewSymbolTable(env)
		for i, atom := range ast {
			switch atom := atom.(type) {
			default:
				return nil, fmt.Errorf("Variable identifier must be a string (was %T)", atom)
			case string:
				if atom == "&" {
					if i+1 == len(ast) {
						return nil, fmt.Errorf("binding list cannot end with &")
					}
					newEnv.Set(ast[i+1].(string), expressions[i:])
					return newEnv, nil
				}
				newEnv.Set(atom, expressions[i])
			}
		}
		return newEnv, nil
	default:
		return nil, fmt.Errorf("Binding must receive an array")
	}
}

type tcoFN struct {
	f          func(args []interface{}) (interface{}, error)
	bodyAST    interface{}
	env        *Environment
	argSpecAST interface{}
}

// type SomeType struct {
// 	A string
// 	I int
// }

// type Getter interface {
// 	Get(string) (interface{}, error)
// }

// type Setter interface {
// 	Set(string, interface{}) error
// }

// type Mapper interface {
// 	Getter
// 	Setter
// }

/*
["def", "get", ["fn", ["a", "b"],
    ["if", ["contains?", "a", "b"], [".-", "a", "b"], null]]],
["def", "set", ["fn", ["a", "b", "c"],
  ["do", [".-", "a", "b", "c"], "a"]]],
["def", "first", ["fn", ["a"],
  ["if", [">", [".-", "a", ["`", "length"]], 0],
    ["nth", "a", 0],
    null]]],
["def", "last", ["fn", ["a"],
  ["nth", "a", ["-", [".-", "a", ["`", "length"]], 1]]]],

*/

// EVAL returns an atom after evaluating an atom entry
func EVAL(ast interface{}, env *Environment) (interface{}, error) {
	for {
		// fmt.Println("(ง'̀-'́)ง", ast)
		switch typedAST := ast.(type) {
		case []interface{}:
			switch first := typedAST[0].(type) {
			case string:
				switch first {

				// apply
				case "def":
					identifier, ok := typedAST[1].(string)
					if !ok {
						return nil, fmt.Errorf("Second argument in def %q must be a string name", typedAST[1])
					}
					value, err := EVAL(typedAST[2], env)
					if err != nil {
						// return nil, fmt.Errorf("Invalid def body for def %q", typedAST[1])
						return nil, err
					}
					env.Set(identifier, value)
					return value, nil
				case "`": // quote
					return typedAST[1], nil
				case ".-": // get/set
					// elements, err := evalAST(typedAST[1:], env)
					// if err != nil {
					// 	return nil, err
					// }
					// switch elements := elements.(type) {
					// case []interface{}:
					// 	index, ok := elements[1].(string)
					// 	if ok {
					// 		return nil, fmt.Errorf("index in .- must be  string")
					// 	}
					// 	x, err := elements[0].(Mapper).Get(index)
					// 	if err != nil {
					// 		return nil, err
					// 	}
					// 	if len(elements) < 3 {
					// 		// get
					// 		return x, nil
					// 	}
					// 	// set
					// 	elements[0].(Mapper).Set(index, elements[2])
					// 	return elements[2], nil
					// default:
					// }
				case "fn":
					if len(typedAST) != 3 {
						return nil, fmt.Errorf("fn need 2 arguments (found %d)", len(typedAST))
					}
					return tcoFN{
						f: func(args []interface{}) (interface{}, error) {
							newEnv, err := envBind(typedAST[1], env, args)
							if err != nil {
								return nil, err
							}
							return EVAL(typedAST[2], newEnv)
						},
						bodyAST:    typedAST[2],
						env:        env,
						argSpecAST: typedAST[1],
					}, nil

				// TCO
				case "let":
					newEnv := NewSymbolTable(env)
					variables, ok := typedAST[1].([]interface{})
					if !ok {
						return nil, fmt.Errorf("Second argument in let must be a list")
					}
					if len(variables)%2 != 0 {
						return nil, fmt.Errorf("Second argument in let must be a list of pairs of name value")
					}
					for i := range variables {
						if i%2 != 0 {
							continue
						}
						value, err := EVAL(variables[i+1], newEnv)
						if err != nil {
							return nil, err
						}
						_, err = newEnv.Set(variables[i].(string), value)
						if err != nil {
							return nil, err
						}
					}
					env = newEnv
					ast = typedAST[2].([]interface{})
					goto contTCO
				case "if":
					evaledCondition, err := EVAL(typedAST[1], env)
					if err != nil {
						return nil, err
					}
					var ifCondition bool
					switch evaledCondition := evaledCondition.(type) {
					case bool:
						ifCondition = evaledCondition
					case float64: // FIXME: float64 cannot be compared reliably with == / !=
						ifCondition = evaledCondition != 0
					case nil:
						ifCondition = false
					case []interface{}:
						ifCondition = len(evaledCondition) > 0
					case string:
						ifCondition = evaledCondition != ""
					default:
						return nil, fmt.Errorf("if requires a quasi boolean condition but got %T", evaledCondition)
					}

					if ifCondition {
						ast = typedAST[2]
					} else {
						ast = typedAST[3]
					}
					goto contTCO
				case "do":
					if len(typedAST) > 2 {
						_, err := evalAST(typedAST[1:len(typedAST)-1], env)
						if err != nil {
							return nil, err
						}
					}
					ast = typedAST[len(typedAST)-1]
					goto contTCO
				}
			}

			// default cases for both switches
			// -> fnCall(ast, env)
			elements, err := evalAST(typedAST, env)
			if err != nil {
				return nil, err
			}

			switch elements := elements.(type) {
			case []interface{}:
				f := elements[0]
				switch f := f.(type) {
				case func([]interface{}) (interface{}, error):
					return f(elements[1:])
				case tcoFN:
					ast = f.bodyAST
					env, err = envBind(f.argSpecAST, f.env, elements[1:])
					if err != nil {
						return nil, err
					}
					goto contTCO
				default:
					return nil, fmt.Errorf("Non callable atom %T", f)
				}
			default:
				return nil, fmt.Errorf("?? BOGUS %T", elements)
			}
		default:
			return evalAST(ast, env)
		}
	contTCO:
		// fmt.Println("( '̀-'́) ", ast)
	}
}

// PRINT prints the atom out
func PRINT(ast interface{}) ([]byte, error) {
	return json.Marshal(ast)
	// spew.Dump(ast)
	// return nil, nil
}

// REPL calls READ -> EVAL -> PRINT
func REPL(in []byte, env *Environment) ([]byte, error) {
	if len(in) == 0 {
		return []byte{}, nil
	}

	atom, err := READ(in)
	if err != nil {
		return nil, err
	}

	out, err := EVAL(atom, env)
	if err != nil {
		return nil, err
	}

	b, err := PRINT(out)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func main() {
	if len(os.Args) >= 2 {
		symbolTable := BaseSymbolTable()
		symbolTable.Set("ARGS", os.Args[2:])

		args := make([]interface{}, len(os.Args))
		for i := range os.Args {
			args[i] = os.Args[i]
		}
		_, err := EVAL(
			[]interface{}{
				"load", []interface{}{
					"`", args[1],
				},
			},
			symbolTable)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
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

			b, err := REPL([]byte(strings.Trim(line, " \t\n")), symbolTable)
			if err != nil {
				fmt.Println(err)
				continue
			}

			fmt.Println(string(b))
		}
	}
}