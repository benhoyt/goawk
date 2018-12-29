BEGIN {
	names[1] = "s"
	names[2] = "m"
	for (i in names) {
		x[names[i]] = i
		print i, names[i], x[names[i]]
	}
	print x["s"]
#	adump(x)
#	adump(names)
}
