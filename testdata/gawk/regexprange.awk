BEGIN {
	range = "[a-dx-z]"

	split("ABCDEFGHIJKLMNOPQRSTUVWXYZ", upper, "")
	split("abcdefghijklmnopqrstuvwxyz", lower, "")

	for (i = 1; i in upper; i++)
		printf("%s ~ %s ---> %s\n",
			upper[i], range, (upper[i] ~ range) ? "true" : "false")

	for (i = 1; i in lower; i++)
		printf("%s ~ %s ---> %s\n",
			lower[i], range, (lower[i] ~ range) ? "true" : "false")
}
