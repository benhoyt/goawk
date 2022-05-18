
# GoAWK: an AWK interpreter with CSV support

[![Documentation](https://pkg.go.dev/badge/github.com/benhoyt/goawk)](https://pkg.go.dev/github.com/benhoyt/goawk)
[![GitHub Actions Build](https://github.com/benhoyt/goawk/workflows/Go/badge.svg)](https://github.com/benhoyt/goawk/actions?query=workflow%3AGo)


AWK is a fascinating text-processing language, and somehow after reading the delightfully-terse [*The AWK Programming Language*](https://ia802309.us.archive.org/25/items/pdfy-MgN0H1joIoDVoIC7/The_AWK_Programming_Language.pdf) I was inspired to write an interpreter for it in Go. So here it is, feature-complete and tested against "the one true AWK" and GNU AWK test suites.

GoAWK is a POSIX-compatible version of AWK, and additionally has a CSV mode for reading and writing CSV and TSV files. This feature was sponsored by the [library of the University of Antwerp](https://www.uantwerpen.be/en/library/). Read the [CSV documentation](https://github.com/benhoyt/goawk/blob/master/csv.md).

You can also read one of the articles I've written about GoAWK:

* The original article about [how GoAWK works and performs](https://benhoyt.com/writings/goawk/)
* How I converted the tree-walking interpreter to a [bytecode compiler and virtual machine](https://benhoyt.com/writings/goawk-compiler-vm/)
* A description of why and how I added [CSV support](https://benhoyt.com/writings/goawk-csv/)


## Basic usage

To use the command-line version, simply use `go install` to install it, and then run it using `goawk` (assuming `~/go/bin` is in your `PATH`):

```shell
$ go install github.com/benhoyt/goawk@latest

$ goawk 'BEGIN { print "foo", 42 }'
foo 42

$ echo 1 2 3 | goawk '{ print $1 + $3 }'
4

# Or use GoAWK's CSV and @"named-field" support:
$ echo -e 'name,amount\nBob,17.50\nJill,20\n"Boba Fett",100.00' | \
  goawk -i csv -H '{ total += @"amount" } END { print total }'
137.5
```

On Windows, `"` is the shell quoting character, so use `"` around the entire AWK program on the command line, and use `'` around AWK strings -- this is a non-POSIX extension to make GoAWK easier to use on Windows:

```powershell
C:\> goawk "BEGIN { print 'foo', 42 }"
foo 42
```

To use it in your Go programs, you can call `interp.Exec()` directly for simple needs:

```go
input := strings.NewReader("foo bar\n\nbaz buz")
err := interp.Exec("$0 { print $1 }", " ", input, nil)
if err != nil {
    fmt.Println(err)
    return
}
// Output:
// foo
// baz
```

Or you can use the `parser` module and then `interp.ExecProgram()` to control execution, set variables, and so on:

```go
src := "{ print NR, tolower($0) }"
input := "A\naB\nAbC"

prog, err := parser.ParseProgram([]byte(src), nil)
if err != nil {
    fmt.Println(err)
    return
}
config := &interp.Config{
    Stdin: strings.NewReader(input),
    Vars:  []string{"OFS", ":"},
}
_, err = interp.ExecProgram(prog, config)
if err != nil {
    fmt.Println(err)
    return
}
// Output:
// 1:a
// 2:ab
// 3:abc
```

If you need to repeat execution of the same program on different inputs, you can call [`interp.New`](https://pkg.go.dev/github.com/benhoyt/goawk/interp#New) once, and then call the returned object's `Execute` method as many times as you need.

Read the [package documentation](https://pkg.go.dev/github.com/benhoyt/goawk) for more details.


## Differences from AWK

The intention is for GoAWK to conform to `awk`'s behavior and to the [POSIX AWK spec](http://pubs.opengroup.org/onlinepubs/9699919799/utilities/awk.html), but this section describes some areas where it's different.

Additional features GoAWK has over AWK:

* It has proper support for CSV and TSV files ([read the documentation](https://github.com/benhoyt/goawk/blob/master/csv.md)).
* It supports negative field indexes to access fields from the right, for example, `$-1` refers to the last field.
* It's embeddable in your Go programs! You can even call custom Go functions from your AWK scripts.
* Most AWK scripts are faster than `awk` and on a par with `gawk`, though usually slower than `mawk`. (See [recent benchmarks](https://benhoyt.com/writings/goawk-compiler-vm/#virtual-machine-results).)
* The parser supports `'single-quoted strings'` in addition to `"double-quoted strings"`, primarily to make Windows one-liners easier (the Windows `cmd.exe` shell uses `"` as the quote character).

Things AWK has over GoAWK:

* Scripts that use regular expressions are slower than other implementations (unfortunately Go's `regexp` package is relatively slow).
* AWK is written by Alfred Aho, Peter Weinberger, and Brian Kernighan.


## Stability

This project has a good suite of tests, which include my own intepreter tests, the original AWK test suite, and the relevant tests from the Gawk test suite. I've used it a bunch personally, and it's used in the [Benthos](https://github.com/benthosdev/benthos) stream processor as well as by the software team at the library of the University of Antwerp. However, to `err == human`, so please use GoAWK at your own risk. I intend not to change the Go API in a breaking way in any v1.x.y version.


## AWKGo

The GoAWK repository also includes the creatively-named AWKGo, an AWK-to-Go compiler. This is experimental and is not subject to the stability requirements of GoAWK itself. You can [read more about AWKGo](https://benhoyt.com/writings/awkgo/) or browse the code on the [`awkgo` branch](https://github.com/benhoyt/goawk/tree/awkgo/awkgo).


## License

GoAWK is licensed under an open source [MIT license](https://github.com/benhoyt/goawk/blob/master/LICENSE.txt).


## The end

Have fun, and please [contact me](https://benhoyt.com/) if you're using GoAWK or have any feedback!
