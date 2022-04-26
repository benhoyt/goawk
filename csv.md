
# GoAWK's CSV and TSV file support

[CSV](https://en.wikipedia.org/wiki/Comma-separated_values) and [TSV](https://en.wikipedia.org/wiki/Tab-separated_values) files are often used in data processing today, but unfortunately you can't properly process them using POSIX AWK. You can change the field separator to `,` or tab (for example `awk -F,` or `awk '-F\t'`) but that doesn't handle quoted or multi-line fields.

There are other workarounds, such as [Gawk's FPAT feature](https://www.gnu.org/software/gawk/manual/html_node/Splitting-By-Content.html), various [CSV extensions](http://mcollado.z15.es/xgawk/) for Gawk, or Adam Gordon Bell's [csvquote](https://github.com/adamgordonbell/csvquote) tool. There's also [frawk](https://github.com/ezrosent/frawk), which is an amazing tool that natively supports CSV, but unfortunately it deviates quite a bit from POSIX-compatible AWK.

Since version vTODO, GoAWK has included proper CSV support, which allows you to read and write CSV and TSV files, including proper handling of quoted and multi-line fields as per [RFC 4180](https://rfc-editor.org/rfc/rfc4180.html).

In addition, GoAWK supports a special "named field" construct that allows you to access CSV fields by name as well as number, for example `@"Address"` rather than `$5`.

Keep reading for full documentation, or skip straight to the [examples](#examples).

**Many thanks to the [library of the University of Antwerp](https://www.uantwerpen.be/en/library/), who sponsored this feature in April 2022.**


## CSV input configuration

To enable CSV input mode when using the `goawk` command, use the `-i mode` command line argument, where `mode` is either `csv` or `tsv` followed by optional configuration. You can also set the `INPUTMODE` special variable in the `BEGIN` block.

The full syntax of `mode` is as follows:

```
csv|tsv [separator=<char>] [comment=<char>] [noheader]
```

The first field in `mode` is the format: `csv` for comma-separated values or `tsv` for tab-separated values. Optionally following the mode are configuration fields, defined as follows:

* `separator=<char>`: override the separator character to `<char>`, for example `separator=|` to use the pipe character. The default is `,` for `csv` mode or `\t` (tab) for `tsv` mode.
* `comment=<char>`: consider lines starting with `<char>` to be comments and ignore them, for example `comment=#` will ignore any lines starting with `#`. The default is not to support comments.
* `noheader`: don't treat the first line in each input file as a header row. The default is to treat the first line as a header row, skipping regular processing for the row but providing the field names for [`@"named-field"` syntax](#named-field-access). If `noheader` is specified, you can't use named field access.



## CSV output configuration

TODO


## Named field access

As already shown above, ...

@"Address"

TODO: you can use it with dynamic field names
TODO: assignment of @"foo" not yet supported.
Cannot use if `noheader` mode.


## The `FIELDS` array

Updated as each file is opened. Not set if `noheader` mode.


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

To output only a single field (the two-letter abbreviation):

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

To convert the file from CSV to TSV format (note the use of `noheader` so we include the header row in the output):

```
$ goawk -i 'csv noheader' -o tsv '{ print $1, $2 }' states.csv >states.tsv
$ head -n3 states.tsv
State   Abbreviation
Alabama AL
Alaska  AK
```

If you don't know the number of fields, you can use a field assignment like `$1=$1` so GoAWK reformats the row to the TSV output format:

```
$ goawk -i 'csv noheader' -o tsv '{ $1=$1; print }' states.csv >states.tsv
$ head -n3 states.tsv
State   Abbreviation
Alabama AL
Alaska  AK
```

We can use GoAWK to add a comment and convert the separator character to `|` (pipe):

```
$ goawk -i 'csv noheader' -o 'csv separator=|' 'BEGIN { print "# comment" } { $1=$1; print }' states.csv >states.psv
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

And finally, four equivalent examples showing different ways to specify the input mode, using `-i` or the `INPUTMODE` special variable (the same technique words for `-o` and `OUTPUTMODE`):

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


##