
# GoAWK's CSV and TSV file support

[CSV](https://en.wikipedia.org/wiki/Comma-separated_values) and [TSV](https://en.wikipedia.org/wiki/Tab-separated_values) files are often used in data processing today, but unfortunately you can't properly process them using POSIX AWK. You can change the field separator to `,` or tab (for example `awk -F,` or `awk '-F\t'`) but that doesn't handle quoted or multi-line fields.

There are other workarounds, such as [Gawk's FPAT feature](https://www.gnu.org/software/gawk/manual/html_node/Splitting-By-Content.html), various [CSV extensions](http://mcollado.z15.es/xgawk/) for Gawk, or Adam Gordon Bell's [csvquote](https://github.com/adamgordonbell/csvquote) tool. There's also [frawk](https://github.com/ezrosent/frawk), which is an amazing tool that natively supports CSV, but unfortunately it deviates quite a bit from POSIX-compatible AWK.

Since version v1.17.0, GoAWK has included CSV support, which allows you to read and write CSV and TSV files, including proper handling of quoted and multi-line fields as per [RFC 4180](https://rfc-editor.org/rfc/rfc4180.html). In addition, GoAWK supports a "named field" construct that allows you to access CSV fields by name as well as number, for example `@"Address"` rather than `$5`.

**Many thanks to the [library of the University of Antwerp](https://www.uantwerpen.be/en/library/), who sponsored this feature in May 2022.** Thanks also to [Eli Rosenthal](https://github.com/ezrosent), whose frawk tool inspired aspects of the design (including the `-i` and `-o` command line arguments).

Links to sections:

* [CSV input configuration](#csv-input-configuration)
* [CSV output configuration](#csv-output-configuration)
* [Named field syntax](#named-field-syntax)
* [Go API](#go-api)
* [Examples](#examples)
* [Examples based on csvkit](#examples-based-on-csvkit)
* [Performance](#performance)
* [Future work](#future-work)


## CSV input configuration

When in CSV input mode, GoAWK ignores the regular field and record separators (`FS` and `RS`), instead parsing input into records and fields using the CSV or TSV format. Fields can be accessed using the standard AWK numbered field syntax (for example, `$1` or `$5`), or using the GoAWK-specific [named field syntax](#named-field-syntax).

To enable CSV input mode when using the `goawk` program, use the `-i mode` command line argument. You can also enable CSV input mode by setting the `INPUTMODE` special variable in the `BEGIN` block, or by using the [Go API](#go-api). The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>] [comment=<char>] [header]
```

The first field in `mode` is the format: `csv` for comma-separated values or `tsv` for tab-separated values. Optionally following the mode are configuration fields, defined as follows:

* `separator=<char>`: override the separator character, for example `separator=|` to use the pipe character. The default is `,` (comma) for `csv` format or `\t` (tab) for `tsv` format.
* `comment=<char>`: consider lines starting with the given character to be comments and skip them, for example `comment=#` will ignore any lines starting with `#` (without preceding whitespace). The default is not to support comments.
* `header`: treat the first line of each input file as a header row providing the field names, and enable the `@"field"` syntax as well as the `FIELDS` array. This option is equivalent to the `-H` command line argument. If neither `header` or `-H` is specified, you can't use named fields.



## CSV output configuration

When in CSV output mode, the GoAWK `print` statement with one or more arguments ignores `OFS` and `ORS` and separates its arguments (fields) and records using CSV formatting. No header row is printed; if required, a header row can be printed in the `BEGIN` block manually. No other functionality is changed, for example, `printf` doesn't do anything different in CSV output mode.

**NOTE:** The behaviour of `print` without arguments remains unchanged. This means you can print the input line (`$0`) without further quoting by using a bare `print` statement, but `print $0` will print the input line as a single CSV field, which is probably not what you want. See the [example](#example-convert-between-formats-all-fields) below.

To enable CSV output mode when using the `goawk` program, use the `-o mode` command line argument. You can also enable CSV output mode by setting the `OUTPUTMODE` special variable in the `BEGIN` block, or by using the [Go API](#go-api). The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>]
```

The first field in `mode` is the format: `csv` for comma-separated values or `tsv` for tab-separated values. Optionally following the mode are configuration fields, defined as follows:

* `separator=<char>`: override the separator character, for example `separator=|` to use the pipe character. The default is `,` (comma) for `csv` format or `\t` (tab) for `tsv` format.


## Named field syntax

If the `header` option or `-H` argument is given, CSV input mode parses the first row of each input file as a header row containing a list of field names.

When the header option is enabled, you can use the GoAWK-specific "named field" operator (`@`) to access fields by name instead of by number (`$`). For example, given the header row `id,name,email`, for each record you can access the email address using `@"email"`, `$3`, or even `$-1` (first field from the right). Further usage examples are shown [below](#examples).

Every time a header row is processed, the `FIELDS` special array is updated: it is a mapping of field number to field name, allowing you to loop over the field names dynamically. For example, given the header row `id,name,email`, GoAWK sets `FIELDS` using the equivalent of:

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

See the [full reference documentation](https://pkg.go.dev/github.com/benhoyt/goawk/interp#Config) for the `interp.Config` struct.


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

### Example: output a field by name

To output a field by name (in this case the state's abbreviation):

```
$ goawk -i csv -H '{ print @"Abbreviation" }' testdata/csv/states.csv
AL
AK
AZ
...
```

### Example: match a field and count

To count the number of states that have "New" in the name, and then print out what they are:

```
$ goawk -i csv -H '@"State" ~ /New/ { n++ } END { print n }' testdata/csv/states.csv
4
$ goawk -i csv -H '@"State" ~ /New/ { print @"State" }' testdata/csv/states.csv
New Hampshire
New Jersey
New Mexico
New York
```

### Example: rename and reorder fields

To rename and reorder the fields from `State`, `Abbreviation` to `abbr`, `name`. Note that the `print` statement in the `BEGIN` block prints the header row for the output:

```
$ goawk -i csv -H -o csv 'BEGIN { print "abbr", "name" } { print @"Abbreviation", @"State" }' testdata/csv/states.csv
abbr,name
AL,Alabama
AK,Alaska
...
```

### Example: convert between formats (explicit field list)

To convert the file from CSV to TSV format (note how we're *not* using `-H`, so the header row is included):

```
$ goawk -i csv -o tsv '{ print $1, $2 }' testdata/csv/states.csv
State	Abbreviation
Alabama	AL
Alaska	AK
...
```

### Example: convert between formats (all fields)

If you want to convert between CSV and TSV format but don't know the number of fields, you can use a field assignment like `$1=$1` so that GoAWK reformats `$0` according to the output format (TSV in this case). This is similar to how in POSIX AWK a field assignment reformats `$0` according to the output field separator (`OFS`). Then `print` without arguments prints the raw value of `$0`:

```
$ goawk -i csv -o tsv '{ $1=$1; print }' testdata/csv/states.csv
State	Abbreviation
Alabama	AL
Alaska	AK
...
```

**NOTE:** It's not correct to use `print $0` in this case, because that would print `$0` as a single TSV field, which you generally don't want:

```
$ goawk -i csv -o tsv '{ $1=$1; print $0 }' testdata/csv/states.csv  # INCORRECT!
"State	Abbreviation"
"Alabama	AL"
"Alaska	AK"
...
```

### Example: override separator

To test overriding the separator character, we can use GoAWK to add a comment and convert the separator to `|` (pipe). We'll also add a comment line to test comment handling:

```
$ goawk -i csv -o 'csv separator=|' 'BEGIN { printf "# comment\n" } { $1=$1; print }' testdata/csv/states.csv
# comment
State|Abbreviation
Alabama|AL
Alaska|AK
...
```

### Example: skip comment lines

We can process the "pipe-separated values" file generated above, skipping comment lines, and printing the first three state names (accessed by field number this time):

```
$ goawk -i 'csv header comment=# separator=|' 'NR<=3 { print $1 }' testdata/csv/states.psv
Alabama
Alaska
Arizona
```

### Example: use dynamic field names

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

### Example: use the `FIELDS` array

A somewhat contrived example showing use of the `FIELDS` array:

```
$ cat testdata/csv/fields.csv
id,name,email
1,Bob,b@bob.com
$ goawk -i csv -H '{ for (i=1; i in FIELDS; i++) print i, FIELDS[i] }' testdata/csv/fields.csv
1 id
2 name
3 email
```

### Example: create CSV file from array

The following example shows how you might pull fields out of an integer-indexed array to produce a CSV file:

```
$ goawk -o csv 'BEGIN { print "id", "name"; names[1]="Bob"; names[2]="Jane"; for (i=1; i in names; i++) print i, names[i] }'
id,name
1,Bob
2,Jane
```

### Example: create CSV file by assigning fields

This example shows the same result, but producing the CSV output by assigning individual fields and then using a bare `print` statement:

```
$ goawk -o csv 'BEGIN { print "id", "name"; $1=1; $2="Bob"; print; $1=2; $2="Jane"; print }'
id,name
1,Bob
2,Jane
```

### Example: different ways to specify CSV mode

And finally, four equivalent examples showing different ways to specify the input mode, using `-i` or the `INPUTMODE` special variable (the same techniques work for `-o` and `OUTPUTMODE`):

```
$ goawk -i csv -H '@"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
$ goawk -icsv -H '@"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
$ goawk 'BEGIN { INPUTMODE="csv header" } @"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
$ goawk -v 'INPUTMODE=csv header' '@"State"=="New York" { print @"Abbreviation" }' testdata/csv/states.csv
NY
```


## Examples based on csvkit

The [csvkit](https://csvkit.readthedocs.io/en/latest/index.html) suite is a set of tools that allow you to quickly analyze and extract fields from CSV files. Each csvkit tool allows you to do a specific task; GoAWK is more low-level and verbose, but also a more general tool ([`csvsql`](https://csvkit.readthedocs.io/en/latest/tutorial/3_power_tools.html#csvsql-and-sql2csv-ultimate-power) being the exception!). GoAWK also runs significantly faster than csvkit (the latter is written in Python).

Below are a few snippets showing how you'd do some of the tasks in the csvkit documentation, but using GoAWK (the input file is [testdata/csv/nz-schools.csv](https://github.com/benhoyt/goawk/blob/master/testdata/csv/nz-schools.csv)):

### csvkit example: print column names

```
$ csvcut -n testdata/csv/nz-schools.csv
  1: School_Id
  2: Org_Name
  3: Decile
  4: Total

# In GoAWK you loop through the FIELDS array for this, and you can print the
# data in any format you want:
$ goawk -i csv -H '{ for (i=1; i in FIELDS; i++) printf "%3d: %s\n", i, FIELDS[i]; exit }' testdata/csv/nz-schools.csv
  1: School_Id
  2: Org_Name
  3: Decile
  4: Total
```

### csvkit example: select a subset of columns

```
$ csvcut -c Org_Name,Total testdata/csv/nz-schools.csv
Org_Name,Total
Waipa Christian School,60
Remarkables Primary School,494
...

# In GoAWK you need to print the field names explicitly in BEGIN:
$ goawk -i csv -H -o csv 'BEGIN { print "Org_Name", "Total" } { print @"Org_Name", @"Total" }' testdata/csv/nz-schools.csv
Org_Name,Total
Waipa Christian School,60
Remarkables Primary School,494
...

# But you can also change the column names and reorder them:
$ goawk -i csv -H -o csv 'BEGIN { print "# Students", "School" } { print @"Total", @"Org_Name" }' testdata/csv/nz-schools.csv
# Students,School
60,Waipa Christian School
494,Remarkables Primary School
...
```

### csvkit example: generate statistics

There's no equivalent of the `csvstat` tool in GoAWK, but you can calculate statistics yourself. For example, to calculate the total number of students in New Zealand schools, you can do the following (`csvstat` is giving a warning due to the single-column input):

```
$ csvcut -c Total testdata/csv/nz-schools.csv | csvstat --sum
/usr/local/lib/python3.9/dist-packages/agate/table/from_csv.py:74: RuntimeWarning: Error sniffing CSV dialect: Could not determine delimiter
802,516

$ goawk -i csv -H '{ sum += @"Total" } END { print sum }' testdata/csv/nz-schools.csv
802516
```

To calculate the average (mean) decile level for boys' and girls' schools (sorry, boys!):

```
$ csvgrep -c Org_Name -m Boys testdata/csv/nz-schools.csv | csvcut -c Decile | csvstat --mean
/usr/local/lib/python3.9/dist-packages/agate/table/from_csv.py:74: RuntimeWarning: Error sniffing CSV dialect: Could not determine delimiter
6.45
$ csvgrep -c Org_Name -m Girls testdata/csv/nz-schools.csv | csvcut -c Decile | csvstat --mean
/usr/local/lib/python3.9/dist-packages/agate/table/from_csv.py:74: RuntimeWarning: Error sniffing CSV dialect: Could not determine delimiter
8.889

$ goawk -i csv -H '/Boys/  { d+=@"Decile"; n++ } END { print d/n }' testdata/csv/nz-schools.csv 
6.45
$ goawk -i csv -H '/Girls/ { d+=@"Decile"; n++ } END { print d/n }' testdata/csv/nz-schools.csv 
8.88889
```


## Performance

The performance of GoAWK's CSV input and output mode is quite good, on a par with using the `encoding/csv` package from Go directly, and much faster than the `csv` module in Python. CSV input speed is significantly slower than `frawk`, though CSV output speed is significantly faster than `frawk`.

Below are the results of some simple read and write [benchmarks](https://github.com/benhoyt/goawk/blob/master/scripts/csvbench) using `goawk` and `frawk` as well as plain Python and Go. The output of the write benchmarks is a 1GB, 3.5 million row CSV file with 20 columns (including quoted columns); the input for the read benchmarks uses that same file. Times are in seconds, showing the best of three runs on a 64-bit Linux laptop with an SSD drive:

Test            | goawk | frawk | Python |   Go
--------------- | ----- | ----- | ------ | ----
Reading 1GB CSV |  3.18 |  1.01 |   13.4 | 3.22
Writing 1GB CSV |  5.64 |  13.0 |   17.0 | 3.24


## Future work

* Consider adding a `printrow(a)` or similar function to make it easier to construct CSV rows from scratch.
  - `a` would be an array such as: `a["name"] = "Bob"; a["age"] = 7`
  - keys would be ordered by `OFIELDS` (eg: `OFIELDS[1] = "name"; OFIELDS[2] = "age"`) or by "smart name" if `OFIELDS` not set ("smart name" meaning numeric if `a` keys are numeric, string otherwise)
  - `printrow(a)` could take an optional second `fields` array arg to use that instead of the global `OFIELDS`
* Consider allowing `-H` to accept an optional list of field names which could be used as headers in the absence of headers in the file itself (either `-H=name,age` or `-i 'csv header=name,age'`).
* Consider adding TrimLeadingSpace CSV input option. See: https://github.com/benhoyt/goawk/issues/109
* Consider supporting `@"id" = 42` named field assignment.


## Feedback

Please [open an issue](https://github.com/benhoyt/goawk/issues) if you have bug reports or feature requests for GoAWK's CSV support.
