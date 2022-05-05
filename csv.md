
# GoAWK's CSV and TSV file support

[CSV](https://en.wikipedia.org/wiki/Comma-separated_values) and [TSV](https://en.wikipedia.org/wiki/Tab-separated_values) files are often used in data processing today, but unfortunately you can't properly process them using POSIX AWK. You can change the field separator to `,` or tab (for example `awk -F,` or `awk '-F\t'`) but that doesn't handle quoted or multi-line fields.

There are other workarounds, such as [Gawk's FPAT feature](https://www.gnu.org/software/gawk/manual/html_node/Splitting-By-Content.html), various [CSV extensions](http://mcollado.z15.es/xgawk/) for Gawk, or Adam Gordon Bell's [csvquote](https://github.com/adamgordonbell/csvquote) tool. There's also [frawk](https://github.com/ezrosent/frawk), which is an amazing tool that natively supports CSV, but unfortunately it deviates quite a bit from POSIX-compatible AWK.

Since version v1.17.0, GoAWK has included CSV support, which allows you to read and write CSV and TSV files, including proper handling of quoted and multi-line fields as per [RFC 4180](https://rfc-editor.org/rfc/rfc4180.html).

In addition, GoAWK supports a special "named field" construct that allows you to access CSV fields by name as well as number, for example `@"Address"` rather than `$5`.

**Many thanks to the [library of the University of Antwerp](https://www.uantwerpen.be/en/library/), who sponsored this feature in April 2022.**

Links to sections:

* [CSV input configuration](#csv-input-configuration)
* [CSV output configuration](#csv-output-configuration)
* [Named field syntax](#named-field-syntax)
* [Go API](#go-api)
* [Examples](#examples)
* [Performance](#performance)


## CSV input configuration

When in CSV input mode, GoAWK ignores the regular field and record separators (`FS` and `RS`), instead parsing input into records and fields using the CSV format. Fields can be accessed using the standard AWK numbered field syntax (for example, `$1` or `$5`), or using GoAWK's [named field syntax](#named-field-syntax).

To enable CSV input mode when using the `goawk` program, use the `-i mode` command line argument. You can also set the `INPUTMODE` special variable in the `BEGIN` block, or by using the [Go API](#go-api). The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>] [comment=<char>] [header]
```

The first field in `mode` is the format: `csv` for comma-separated values or `tsv` for tab-separated values. Optionally following the mode are configuration fields, defined as follows:

* `separator=<char>`: override the separator character, for example `separator=|` to use the pipe character. The default is `,` for `csv` mode or `\t` (tab) for `tsv` mode.
* `comment=<char>`: consider lines starting with the given character to be comments and ignore them, for example `comment=#` will ignore any lines starting with `#`. The default is not to support comments.
* `header`: treat the first line of each input file as a header row providing the field names, and enable the `@"field"` syntax and the `FIELDS` array. This option is equivalent to the `-H` command line argument. If neither `header` or `-H` is specified, you can't use named fields.



## CSV output configuration

When in CSV output mode, the GoAWK `print` statement ignores `OFS` and `ORS` and separates fields and records using CSV formatting. No header line is printed -- if required, that must be done in the `BEGIN` block manually. No other functionality is changed, for example, `printf` doesn't do anything different in CSV output mode.

To enable CSV output mode when using the `goawk` program, use the `-o mode` command line argument. You can also set the `OUTPUTMODE` special variable in the `BEGIN` block, or by using the [Go API](#go-api). The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>]
```

The meaning of the fields is the same as for the input mode, except that the only configuration field is `separator`.


## Named field syntax

If the `header` option or `-H` argument is given, CSV input mode parses the first row of each input file as a header row containing a list of field names.

When the header option is enabled, you can use the GoAWK-specific "named field" operator (`@`) to access fields by name instead of by number (`$`). For example, given the header row `id,name,email`, for each record you can access the email address using `@"email"`, `$3`, or even `$-1` (first field from the right). Further usage examples are shown [below](#examples).

Every time a header row is processed, the `FIELDS` special array is updated: it is a mapping of field number to field name, allowing you to loop over the field names dynamically. For example, given the header row `id,name,email`, GoAWK set `FIELDS` using the equivalent of:

```
FIELDS[1] = "id"
FIELDS[2] = "name"
FIELDS[3] = "email"
```

Note that named field assignment such as `@"id" = 42` is not yet supported, but this feature may be added later.


## Go API

When using GoAWK via the Go API, you can still use `INPUTMODE`, but it may be more convenient to use the `interp.Config` fields directly: `InputMode`, `CSVInput`, `OutputMode`, and `CSVOutput`.

Here's a simple snippet showing the use of the `InputMode` and `CSVInput` fields to enable `#` as the comment character:

```
prog, err := parser.ParseProgram([]byte(src), nil)
if err != nil { ... }

config := &interp.Config{
    InputMode: interp.CSVMode,
    CSVInput:  interp.CSVInputConfig{Comment: '#'},
}
_, err = interp.ExecProgram(prog, config)
if err != nil { ... }
```

Note that `INPUTMODE` and `OUTPUTMODE` set using `Vars` or in the `BEGIN` block will override these settings.

See the [full reference documentation](https://pkg.go.dev/github.com/benhoyt/goawk/interp#Config) of the Go API fields.


## Examples

Below are some examples using the [testdata/csv/states.csv](https://github.com/benhoyt/goawk/blob/master/testdata/csv/states.csv) file, which is a simple CSV file whose contents are as follows:

```
"State","Abbreviation"
"Alabama","AL"
"Alaska","AK"
"Arizona","AZ"
"Arkansas","AR"
"California","CA"
...
```

To output only a single field (in this case the state's abbreviation):

```
$ goawk -i csv -H '{ print @"Abbreviation" }' testdata/csv/states.csv
AL
AK
AZ
...
```

To count the number of states that have `New` in the state name (using the named field syntax for `@"State"`):

```
$ goawk -i csv -H '@"State" ~ /New/ { n++ } END { print n }' testdata/csv/states.csv
4
```

To rename and reorder the fields to `abbr`, `name`:

```
$ goawk -i csv -H -o csv 'BEGIN { print "abbr", "name" } { print @"Abbreviation", @"State" }' testdata/csv/states.csv
abbr,name
AL,Alabama
AK,Alaska
...
```

To convert the file from CSV to TSV format (note how we're not using `-H`, so that the header row is included):

```
$ goawk -i csv -o tsv '{ print $1, $2 }' testdata/csv/states.csv
State	Abbreviation
Alabama	AL
Alaska	AK
...
```

If you don't know the number of fields, you can use a field assignment like `$1=$1` so GoAWK reformats the row to the output format (TSV in this case), and then `print` to print the raw value of `$0`:

```
$ goawk -i csv -o tsv '{ $1=$1; print }' testdata/csv/states.csv
State	Abbreviation
Alabama	AL
Alaska	AK
...
```

To test overriding the separator character, we can use GoAWK to add a comment and convert the separator to `|` (pipe). We'll also add a comment line to test comment handling:

```
$ goawk -i csv -o 'csv separator=|' 'BEGIN { printf "# comment\n" } { $1=$1; print }' testdata/csv/states.csv
# comment
State|Abbreviation
Alabama|AL
Alaska|AK
...
```

And then process that "pipe-separated values" file, handling comment lines, and printing the first three state names (accessed by field number this time):

```
$ goawk -i 'csv header comment=# separator=|' 'NR<=3 { print $1 }' testdata/csv/states.psv
Alabama
Alaska
Arizona
```

Similar to the `$` operator, you can also use `@` with dynamic values. For example, if there are fields named `address_1`, `address_2`, up through `address_5`, you could loop over them as follows:

```
$ cat testdata/csv/address5.csv
name,address_1,address_2,address_3,address_4,address_5
Bob Smith,123 Way St,Apt 2B,Township,Cityville,United Plates
$ goawk -i csv -H '{ for (i=1; i<=5; i++) print @("address_" i) }' testdata/csv/address5.csv
123 Way St
Apt 2B
Township
Cityville
United Plates
```

A somewhat contrived example showing use of the `FIELDS` array:

```
$ echo -e 'id,name,email\n1,Bob,b@bob.com' | goawk -i csv -H '{ for (i=1; i in FIELDS; i++) print FIELDS[i] }'
id
name
email
```

And finally, four equivalent examples showing different ways to specify the input mode, using `-i` or the `INPUTMODE` special variable (the same technique works for `-o` and `OUTPUTMODE`):

```
$ goawk -i csv -H '@"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
$ goawk -icsv -H '@"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
$ goawk 'BEGIN { INPUTMODE="csv header" } @"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
$ goawk '-vINPUTMODE=csv header' '@"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
```


## Performance

The performance of GoAWK's CSV input and output mode is quite good, on a par with using the `encoding/csv` package from Go directly, and much faster than the `csv` module in Python. CSV input speed is significantly slower than `frawk`, though CSV output speed is significantly faster than `frawk`.

Below are the results of some simple read and write [benchmarks](https://github.com/benhoyt/goawk/blob/master/scripts/csvbench) using `goawk` and `frawk` as well as plain Python and Go. The input for the read benchmarks is a large 1.5GB, 749,818-row input file with many columns (286). Times are in seconds, showing the best of three runs on a 64-bit Linux laptop with an SSD drive:

Test              | goawk | frawk | python |   go
----------------- | ----- | ----- | ------ | ----
Reading 1.5GB CSV |  6.56 |  2.23 |   22.4 | 7.16 
Writing 0.6GB CSV |  3.42 |  8.01 |   11.7 | 2.22



TODO:
* more clearly document print vs print $0 issue and $1=$1 thing
* add examples for dynamic csv output, setting $1 etc then NF then print
* put ### subheaders above each example
* write Go test to test these examples
* give credit to frawk for some of the design decisions, including the -i/-o options
* think carefully about whether we want a CSVFeatures flag in the parsing config, to enable new, non-backwards compatible features like FIELDS and special vars INPUTMODE/OUTPUTMODE and any other new constructs (@ is okay because it was an error before, so that's backwards-compatible).
  - for reference, I did add ENVIRON in a minor release
  - it's probably okay because people are very unlikely to use these all-UPPERCASE var names
  - however, if we add a new function like "printrow" or (even worse) "output" we probably need it
  - or we can figure out how to make the parser treat "output" as a variable unless it's called
* run design of CSV features, especially {print $0} issue, past Arnold Robbins
