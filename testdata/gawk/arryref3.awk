BEGIN {
	foo(a)

	for (i in a)
		print i, a[i]
}

function foo(b)
{
	a[1] = "global"
	b[2] = "local"
	bar(b)
}

function bar(c)
{
	c = 12
}
