BEGIN {
	s = "Volume 8, Numbers 1-2 / January 1971"
        n = split(s, parts, "[^-A-Za-z0-9]+")
	print "n =", n
	for (i = 1; i <= n; i++)
		printf("s[%d] = \"%s\"\n", i, parts[i])
}
