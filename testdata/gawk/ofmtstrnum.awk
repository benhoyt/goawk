BEGIN {
	split(" 1", f, "|")
	OFMT = "%.1f"
	print f[1]
	x = f[1] + 0
	print f[1]
}
