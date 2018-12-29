BEGIN {
	RS = ""
	FS = ":"
	s = "a:b\nc:d"
	print split(s,a)
	print length(a[2])
}
