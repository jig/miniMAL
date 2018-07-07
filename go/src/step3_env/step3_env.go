// miniMAL
// Copyright (C) 2018 Jordi Íñigo i Griera
// Licensed under MPL 2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Environment struct {
	scope  map[string]interface{}
	parent *Environment
}

// BaseSymbolTable returns a symbol table with predefined contents
func BaseSymbolTable() *Environment {
	return &Environment{
		scope: map[string]interface{}{
			"+": func(args []interface{}) (interface{}, error) {
				result := float64(0)
				for _, v := range args {
					result += v.(float64)
				}
				return result, nil
			},
			"*": func(args []interface{}) (interface{}, error) {
				result := float64(1)
				for _, v := range args {
					result *= v.(float64)
				}
				return result, nil
			},
			"-": func(args []interface{}) (interface{}, error) {
				if err := assertArgNum(args, 2); err != nil {
					return nil, err
				}
				return args[0].(float64) - args[1].(float64), nil
			},
			"/": func(args []interface{}) (interface{}, error) {
				if err := assertArgNum(args, 2); err != nil {
					return nil, err
				}
				return args[0].(float64) / args[1].(float64), nil
			},
		},
	}
}

func assertArgNum(args []interface{}, n int) error {
	if len(args) != n {
		return fmt.Errorf("Invalid number of arguments")
	}
	return nil
}

// NewSymbolTable creates a copy of an environtment table
func NewSymbolTable(parent *Environment) *Environment {
	return &Environment{
		scope:  map[string]interface{}{},
		parent: parent,
	}
}

// Get returns the value of a symbol
func (e *Environment) Get(index string) (interface{}, error) {
	value, ok := e.scope[index]
	if !ok {
		if e.parent == nil {
			return nil, fmt.Errorf("Symbol %q undefined", index)
		}
		return e.parent.Get(index)
	}
	return value, nil
}

// Set defines a new symbol
func (e *Environment) Set(index string, value interface{}) (interface{}, error) {
	e.scope[index] = value
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
		ast = float64(0)
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
		v, err := env.Get(ast)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return ast, nil
	}
}

func apply(
	f interface{},
	args []interface{},
	env *Environment,
) (interface{}, error) {
	switch f := f.(type) {
	case func(args []interface{}) (interface{}, error):
		return f(args)
	default:
		return nil, fmt.Errorf("Non callable atom %T", f)
	}
}

// EVAL returns an atom after evaluating an atom entry
func EVAL(ast interface{}, env *Environment) (interface{}, error) {
	// fmt.Printf("%v\n", ast)
	switch ast := ast.(type) {
	case []interface{}:
		first, ok := ast[0].(string)
		if !ok {
			return nil, fmt.Errorf("First argument must be a string symbol")
		}

		switch first {
		case "def":
			identifier, ok := ast[1].(string)
			if !ok {
				return nil, fmt.Errorf("Second argument in def must be a string name")
			}
			value, err := EVAL(ast[2], env)
			if err != nil {
				return nil, fmt.Errorf("Invalid def body")
			}
			env.Set(identifier, value)
			return value, nil
		case "let":
			newEnv := NewSymbolTable(env)
			variables, ok := ast[1].([]interface{})
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
			return EVAL(ast[2], newEnv)
		default:
			elements, err := evalAST(ast, env)
			if err != nil {
				return nil, err
			}
			switch elements := elements.(type) {
			case []interface{}:
				return apply(elements[0], elements[1:], env)
			default:
				return nil, nil // FIXME
			}
		}
	default:
		return evalAST(ast, env)
	}
}

// PRINT prints the atom out
func PRINT(ast interface{}) ([]byte, error) {
	return json.Marshal(ast)
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
	// b, err := REPL([]byte(` ["let", ["z", ["+", 2, 3]], ["+", 1, "z"]]`), BaseSymbolTable())
	// fmt.Printf("VALUE: %s\nERROR: %v\n", b, err)
	// os.Exit(0)

	reader := bufio.NewReader(os.Stdin)
	symbolTable := BaseSymbolTable()
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
