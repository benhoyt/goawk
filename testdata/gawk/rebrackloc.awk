match($0, /([Nn]ew) Value +[\([]? *([[:upper:]]+)/, f) {
	print "re1", NR, f[1], f[2]
}

match($0, /([][])/, f) {
	print "re2", NR, f[1]
}

/[]]/ {
	print "re3", NR, $0
}

/[\[]/ {
	print "re4", NR, $0
}

/[[]/ {
	print "re5", NR, $0
}

/[][]/ {
	print "re6", NR, $0
}

/[\([][[:upper:]]*/ {
	print "re7", NR, $0
}

/[\([]/ {
	print "re8", NR, $0
}
