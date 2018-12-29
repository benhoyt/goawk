BEGIN {
	x[65]
	for (i in x) {
		# i should be a string
		printf "%c\n", i	# should print 1st char of string
	}
}
