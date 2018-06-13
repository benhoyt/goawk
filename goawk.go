// Implementation of (some of) AWK written in Go
package main

/*
TODO:
- figure out how to represent values (numbers, strings, interface{}?)
- make Interp type an move LINE and FIELDS to members
- have Execute and Evaluate return errors
- add other expressions:
    post inc/dec
    pre inc/dec
    exponentiation
    unary not, plus, minus
    mul, div, mod
    add, sub
    string concat
    equality and inequality
    regex and regex not
    "in" and multi-dimensional "in"
    logical and
    logical or
    cond ?:
    assignments
- variables
- statements

OTHER:
* support for assigning $0 and $1...
* support for assigning RS (instead of newline)
* ENVIRON built-in variable

*/

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "strings"
)

var (
    LINE string
    FIELDS []string
)

func main() {
    if len(os.Args) <= 1 {
        fmt.Fprintf(os.Stderr, "usage: goawk src [filename]\n")
        os.Exit(3)
    }
    src := os.Args[1]

    var err error
    if len(os.Args) <= 2 {
        err = Run(src, os.Stdin)
    } else {
        filename := os.Args[2]
        f, errOpen := os.Open(filename)
        if errOpen != nil {
            fmt.Fprintf(os.Stderr, "can't open file %q\n", filename)
            os.Exit(2)
        }
        defer f.Close()
        err = Run(src, f)
    }
    if err != nil {
        fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
        os.Exit(1)
    }
}

func Run(src string, input io.Reader) error {
    prog, err := Parse(src)
    if err != nil {
        return err
    }
    fmt.Println(prog)
    fmt.Println("-----")

    for _, ss := range prog.Begin {
        Executes(ss)
    }

    scanner := bufio.NewScanner(input)
    NR := 1
    for scanner.Scan() {
        line := scanner.Text()
        fields := strings.Fields(line)
        //NF := len(fields)
        //fmt.Println("LINE:", NR, NF, fields)

        LINE = line
        FIELDS = fields

        for _, a := range prog.Actions {
            pattern := Evaluate(a.Pattern)
            if ToBool(pattern) {
                Executes(a.Stmts)
            }
        }

        NR++
    }
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("reading lines from input: %s", err)
    }

    for _, ss := range prog.End {
        Executes(ss)
    }

    return nil
}

func Parse(src string) (*Program, error) {
    program := &Program{
        Actions: []Action{
            {
                Pattern: &BinaryExpr{
                    Left: &FieldExpr{&ConstExpr{0.0}},
                    Op: "!=",
                    Right: &ConstExpr{""},
                },
                Stmts: []Stmt{
                    &PrintStmt{
                        Args: []Expr{
                            &FieldExpr{&ConstExpr{1.0}},
                            &BinaryExpr{
                                Left: &FieldExpr{&ConstExpr{1.0}},
                                Op: "",
                                Right: &FieldExpr{&ConstExpr{2.0}},
                            },
                        },
                    },
                },
            },
        },
    }
    return program, nil
}
