
# GoAWK's CSV and TSV file support

[CSV](https://en.wikipedia.org/wiki/Comma-separated_values) and [TSV](https://en.wikipedia.org/wiki/Tab-separated_values) files are often used in data processing today, but unfortunately you can't properly process them using POSIX AWK. You can change the field separator to `,` or tab (for example `awk -F,` or `awk '-F\t'`) but that doesn't handle quoted or multi-line fields.

There are other workarounds, such as [Gawk's FPAT feature](https://www.gnu.org/software/gawk/manual/html_node/Splitting-By-Content.html), various [CSV extensions](http://mcollado.z15.es/xgawk/) for Gawk, or Adam Gordon Bell's [csvquote](https://github.com/adamgordonbell/csvquote) tool. There's also [frawk](https://github.com/ezrosent/frawk), which is an amazing tool that natively supports CSV, but unfortunately it deviates quite a bit from POSIX-compatible AWK.

Since version v1.17.0, GoAWK has included CSV support, which allows you to read and write CSV and TSV files, including proper handling of quoted and multi-line fields as per [RFC 4180](https://rfc-editor.org/rfc/rfc4180.html).

In addition, GoAWK supports a special "named field" construct that allows you to access CSV fields by name as well as number, for example `@"Address"` rather than `$5`.

Keep reading for full documentation, or skip straight to the [examples](#examples).

**Many thanks to the [library of the University of Antwerp](https://www.uantwerpen.be/en/library/), who sponsored this feature in April 2022.**


## CSV input configuration

When in CSV input mode, GoAWK ignores the regular field and record separators (`FS` and `RS`), instead parsing input into records and fields using the CSV format. Fields can be accessed using the standard AWK numbered field syntax (for example, `$1` or `$5`), or using GoAWK's [named field syntax](#named-field-syntax).

To enable CSV input mode when using the `goawk` program, use the `-i mode` command line argument. You can also set the `INPUTMODE` special variable in the `BEGIN` block, or by using the [Go API](#go-api). The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>] [comment=<char>] [noheader]
```

The first field in `mode` is the format: `csv` for comma-separated values or `tsv` for tab-separated values. Optionally following the mode are configuration fields, defined as follows:

* `separator=<char>`: override the separator character, for example `separator=|` to use the pipe character. The default is `,` for `csv` mode or `\t` (tab) for `tsv` mode.
* `comment=<char>`: consider lines starting with the given character to be comments and ignore them, for example `comment=#` will ignore any lines starting with `#`. The default is not to support comments.
* `noheader`: don't treat the first line in each input file as a header row. The default is to treat the first line as a header row, skipping regular processing for the row but providing the field names for `@"named-field"` syntax. If `noheader` is specified, you can't use named fields.



## CSV output configuration

When in CSV output mode, the GoAWK `print` statement ignores `OFS` and `ORS` and separates fields and records using CSV formatting. No header line is printed -- if required, that must be done in the `BEGIN` block manually. No other functionality is changed, for example, `printf` doesn't do anything different in CSV output mode.

To enable CSV output mode when using the `goawk` program, use the `-o mode` command line argument. You can also set the `OUTPUTMODE` special variable in the `BEGIN` block, or by using the [Go API](#go-api). The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>]
```

The meaning of the fields is the same as for the input mode, except that the only configuration field is `separator`.


## Named field syntax

CSV input mode automatically parses the first row of each input file as a header row containing a list of field names. To disable special handling of the first row, use the `noheader` option described above.

You can use the GoAWK-specific "named field" operator (`@`) to access fields by name instead of by number (`$`). For example, given the header row `id,name,email`, for each record you can access the email address using `@"email"`, `$3`, or even `$-1` (first field from the right). Further usage examples are shown [below](#examples).

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
$ goawk -i csv '{ print @"Abbreviation" }' states.csv
AL
AK
AZ
...
```

To count the number of states that have `New` in the state name (using the named field syntax for `@"State"`):

```
$ goawk -i csv '@"State" ~ /New/ { n++ } END { print n }' states.csv
4
```

To rename and reorder the fields to `abbr`, `name`:

```
$ goawk -i csv -o csv 'BEGIN { print "abbr", "name" } { print @"Abbreviation", @"State" }' states.csv
abbr,name
AL,Alabama
AK,Alaska
...
```

To convert the file from CSV to TSV format (note the use of `noheader` so we include the header row in the output):

```
$ goawk -i 'csv noheader' -o tsv '{ print $1, $2 }' states.csv
State   Abbreviation
Alabama AL
Alaska  AK
...
```

If you don't know the number of fields, you can use a field assignment like `$1=$1` so GoAWK reformats the row to the output format (TSV in this case):

```
$ goawk -i 'csv noheader' -o tsv '{ $1=$1; print }' states.csv
State   Abbreviation
Alabama AL
Alaska  AK
...
```

To test overriding the separator character, we can use GoAWK to add a comment and convert the separator to `|` (pipe):

```
$ goawk -i 'csv noheader' -o 'csv separator=|' 'BEGIN { printf "# comment\n" } { $1=$1; print }' states.csv >states.psv
$ head -n4 states.psv
# comment
State|Abbreviation
Alabama|AL
Alaska|AK
```

And then process that "pipe-separated values" file, handling comment lines, and printing the first three state names (accessed by field number this time):

```
$ goawk -i 'csv comment=# separator=|' 'NR<=3 { print $1 }' states.psv
Alabama
Alaska
Arizona
```

Similar to the `$` operator, you can also use `@` with dynamic values. For example, if there are fields named `address_1`, `address_2`, up through `address_5`, you could loop over them as follows:

```
$ cat testdata/csv/address5.csv
name,address_1,address_2,address_3,address_4,address_5
Bob Smith,123 Way St,Apt 2B,Township,Cityville,United Plates
$ goawk -i csv '{ for (i=1; i<=5; i++) print @("address_" i) }' testdata/csv/address5.csv
123 Way St
Apt 2B
Township
Cityville
United Plates
```

A somewhat contrived example showing use of the `FIELDS` array:

```
$ echo -e 'id,name,email\n1,Bob,b@bob.com' | goawk -i csv '{ for (i=1; i in FIELDS; i++) print FIELDS[i] }'
id
name
email
```

And finally, four equivalent examples showing different ways to specify the input mode, using `-i` or the `INPUTMODE` special variable (the same technique works for `-o` and `OUTPUTMODE`):

```
$ goawk -i csv '@"State"=="New York" { print @"Abbreviation" }' states.csv
NY
$ goawk -icsv '@"State"=="New York" { print @"Abbreviation" }' states.csv
NY
$ goawk 'BEGIN { INPUTMODE="csv" } @"State"=="New York" { print @"Abbreviation" }' states.csv
NY
$ goawk -vINPUTMODE=csv '@"State"=="New York" { print @"Abbreviation" }' states.csv
NY
```
