function f(a,	i) {
	for (i in a)
		delete a[i]
	if (a == 0)
		print 7
}

BEGIN {
	f(arr)
}
