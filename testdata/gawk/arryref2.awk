BEGIN {
	foo(a)

	for (i in a)
		print i, a[i]
}

function foo(b)
{
	bar(b)
	b[2] = "local"
}

function bar(c)
{
	a[3] = "global"
	c[1] = "local2"
}
