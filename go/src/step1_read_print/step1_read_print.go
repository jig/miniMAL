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

// Atom contains any valid AST
type Atom interface{}

// READ parses a JSON encoded string and unmarshals it to an Atom
func READ(b []byte) (ast Atom, err error) {
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
		astAlloc := &map[string]interface{}{}
		ast = &astAlloc
	case '[':
		astAlloc := []interface{}{}
		ast = &astAlloc
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		var valueAlloc int64
		ast = &valueAlloc
	case '"':
		var valueAlloc string
		ast = &valueAlloc
	default:
		err = fmt.Errorf("Cannot unmarshal: %s", string(b))
	}
	err = json.Unmarshal(b, ast)
	if err != nil {
		return nil, err
	}
	return ast, nil
}

// EVAL returns an atom after evaluating an atom entry
func EVAL(ast Atom, env string) (Atom, error) {
	return ast, nil
}

// PRINT prints the atom out
func PRINT(ast Atom) ([]byte, error) {
	return json.Marshal(ast)
}

// REPL calls READ -> EVAL -> PRINT
func REPL(in []byte) ([]byte, error) {
	if len(in) == 0 {
		return []byte{}, nil
	}

	atom, err := READ(in)
	if err != nil {
		return nil, err
	}

	out, err := EVAL(atom, "")
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
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			os.Exit(0)
		}

		b, err := REPL([]byte(strings.Trim(line, " \t\n")))
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println(string(b))
	}
}
