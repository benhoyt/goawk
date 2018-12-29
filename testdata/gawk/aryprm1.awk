function f(a) {
	if (3 in a)
		print 7
	a = 5
}

BEGIN {
	f(arr)
}
