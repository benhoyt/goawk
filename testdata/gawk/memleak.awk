# This program doesn't do anything except allow us to
# check for memory leak from using a user-supplied
# sorting function.
#
# From Andrew Schorr.

function my_func(i1, v1, i2, v2) {
	return v2-v1
}

BEGIN {
	a[1] = "3"
	a[2] = "2"
	a[3] = "4"
	for (i = 0; i < 10000; i++) {
		n = asort(a, b, "my_func")
		s += n
	}
	print s
}
