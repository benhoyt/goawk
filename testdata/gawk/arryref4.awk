BEGIN {
	foo(a)

	print  a
}

function foo(b)
{
	a = "global"
	b[2] = "local"
#	bar(b)
}

function bar(c)
{
	c = 12
}
