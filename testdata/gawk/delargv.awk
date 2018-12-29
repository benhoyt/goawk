BEGIN {
	ARGV[1] = "/dev/null"
	ARGV[2] = "/dev/null"
	ARGV[3] = "/dev/null"
	ARGC = 4
	delete ARGV[2]
}

END {
	for (i in ARGV)
		printf("length of ARGV[%d] is %d\n", i, length(ARGV[i]))
}
