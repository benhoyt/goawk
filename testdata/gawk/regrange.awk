# Tests due to John Haque, May 2011
#
# The following should be fatal; can't catch them inside awk, though
# $> echo 'a' | ./gawk '/[z-a]/ { print }'
# $> echo 'A' | ./gawk '/[+-[:digit:]]/'

BEGIN {
	char[1] = "."
	pat[1] = "[--\\/]"

	char[2] = "a"
	pat[2] = "[]-c]"

	char[3] = "c"
	pat[3] = "[[a-d]"

	char[4] = "\\"
	pat[4] = "[\\[-\\]]"

	char[5] = "[.c.]"
	pat[5] = "[a-[.e.]]"

	char[6] = "[.d.]"
	pat[6] = "[[.c.]-[.z.]]"

	for (i = 1; i in char; i++) {
		printf("\"%s\" ~ /%s/ --> %d\n", char[i], pat[i],
			char[i] ~ pat[i])
	}
}
