# GoAWK: an AWK interpreter written in Go

[![Documentation](https://pkg.go.dev/badge/github.com/benhoyt/goawk)](https://pkg.go.dev/github.com/benhoyt/goawk)
[![GitHub Actions Build](https://github.com/benhoyt/goawk/workflows/Go/badge.svg)](https://github.com/benhoyt/goawk/actions?query=workflow%3AGo)


AWK is a fascinating text-processing language, and somehow after reading the delightfully-terse [*The AWK Programming Language*](https://ia802309.us.archive.org/25/items/pdfy-MgN0H1joIoDVoIC7/The_AWK_Programming_Language.pdf) I was inspired to write an interpreter for it in Go. So here it is, feature-complete and tested against "the one true AWK" test suite.

[**Read more about how GoAWK works and performs here.**](https://benhoyt.com/writings/goawk/)

## Basic usage

To use the command-line version, simply use `go install` to install it, and then run it using `goawk` (assuming `$GOPATH/bin` is in your `PATH`):

```shell
$ go install github.com/benhoyt/goawk@latest
$ goawk 'BEGIN { print "foo", 42 }'
foo 42
$ echo 1 2 3 | goawk '{ print $1 + $3 }'
4
```

On Windows, `"` is the shell quoting character, so use `"` around the entire AWK program on the command line, and use `'` around AWK strings -- this is a non-POSIX extension to make GoAWK easier to use on Windows:

```powershell
C:\> goawk "BEGIN { print 'foo', 42 }"
foo 42
```

To use it in your Go programs, you can call `interp.Exec()` directly for simple needs:

```go
input := bytes.NewReader([]byte("foo bar\n\nbaz buz"))
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
    Stdin: bytes.NewReader([]byte(input)),
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

Read the [documentation](https://pkg.go.dev/github.com/benhoyt/goawk) for more details.

## Differences from AWK

The intention is for GoAWK to conform to `awk`'s behavior and to the [POSIX AWK spec](http://pubs.opengroup.org/onlinepubs/9699919799/utilities/awk.html), but this section describes some areas where it's different.

Additional features GoAWK has over AWK:

* It's embeddable in your Go programs! You can even call custom Go functions from your AWK scripts.
* I/O-bound AWK scripts (which is most of them) are significantly faster than `awk`, and on a par with `gawk` and `mawk`.
* The parser supports `'single-quoted strings'` in addition to `"double-quoted strings"`, primarily to make Windows one-liners easier (the Windows `cmd.exe` shell uses `"` as the quote character).

Things AWK has over GoAWK:

* CPU-bound AWK scripts are slightly slower than `awk`, and about twice as slow as `gawk` and `mawk`.
* AWK is written by Brian Kernighan.

## Stability

This project has a good suite of tests, and I've used it a bunch personally, but it's certainly not battle-tested or heavily used, so please use at your own risk. I intend not to change the Go API in a breaking way.

## License

GoAWK is licensed under an open source [MIT license](https://github.com/benhoyt/goawk/blob/master/LICENSE.txt).

## The end

Have fun, and please [contact me](https://benhoyt.com/) if you're using GoAWK or have any feedback!


TODO:

awk fflush(stdout)
* before system()
* in printstat() -- what does this do?
* in openfile() when opening a file or pipe or getline
* in getline() before getting line
* fflush(fp) in awkprintf

gawk:
* flush_io() to flush all output before system()
* flush_io() to flush all output before creating new pipe
* fflush(output_fp) before printing an error message to stderr
* fflush(stderr) after printing an error
* builtin.c:efwrite: fflush(fp) if output_is_tty and some other conditions met
* builtin.c:do_printf and do_print

mawk:
* bi_funct.c: bi_fflush(): built-in fflush() function
* bi_funct.c: bi_system(): flush_all_output() before fork
* bi_funct.c: bi_system(): if exec error, fflush(stderr) after printing error message
* bi_funct.c: bi_getline(): built-in getline() calls file_find() if type is F_IN or PIPE_IN
* files.c: file_flush(STRING * sval): flushes named file or all files if sval==""
* files.c: get_pipe(): fflush(stdout); fflush(stderr) - "to keep output ordered correctly"
* files.c: file_find(sval, type): calls get_pipe() if stream not yet created and type is PIPE_OUT or PIPE_IN (i.e., real pipe, not file)
* files.c: file_find() calls tfopen() if type is F_TRUNC or F_APPEND
* files.c: tfopen(): fopen() but no buffering if isatty(fd)
* print.c: bi_print(): built-in print() calls file_find() when redirecting
* print.c: bi_printf(): built-in printf() calls file_find() when redirecting
* fin.c: FINdopen(fd, main_flag): calls isatty(fd) and sets fin->fp based on that and other conditions


// add test that ensures output is flushed before system()
// add test that ensures output is flushed before getline reading from stdin
./goawk 'BEGIN { print "press key: "; getline x; print "X:", x }'

