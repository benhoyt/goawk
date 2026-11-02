# GoAWK's handling of Unicode and bytes

## Bytes vs chars mode

By default, the string functions count bytes, though GoAWK handles UTF-8 encoded strings fine. Specifically:

- `index()` returns the position in bytes
- `length()` returns the string length in bytes
- `match()` sets `RSTART` and `RLENGTH` in bytes
- `printf "%c"` outputs a single byte (the low 8 bits of a number or the first byte of a string)
- `printf "%s"` width and precision count bytes, so precision may split a Unicode character
- `substr()` takes the position and length in bytes, so it may split a Unicode character

Normally this is not a problem, because you usually use the return value of `index` or `length` (and `RSTART` and `RLENGTH`) as the input to `substr`, so if both are in bytes, it works out.

However, if you want to use Unicode characters instead, use `goawk -c` or set `interp.Config.Chars`. In that case, those functions take or return Unicode character positions, `printf "%c"` operates on a Unicode codepoint, and `printf "%s"` width and precision count Unicode codepoints.

GoAWK ignores the `LC_*` environment variables such as `LC_ALL` and `LC_CTYPE`.

## Regular expressions

Regular expressions use Go's UTF-8 based `regexp` package in both modes. A pattern such as `/^.$/` will match a multi-byte character even when `length()` reports more than one byte.

In the following example, the columns are string length, matched length, and whether the entire string matches `/^.$/`:

```shell
$ goawk 'BEGIN { s="╋"; match(s, /./); print length(s), RLENGTH, (s ~ /^.$/) }'
3 3 1
$ goawk -c 'BEGIN { s="╋"; match(s, /./); print length(s), RLENGTH, (s ~ /^.$/) }'
1 1 1
```
