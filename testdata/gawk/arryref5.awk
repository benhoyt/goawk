BEGIN {
	foo(a)

	print  a
}

function foo(b)
{
	b[2] = "local"
	a = "global"
#	bar(b)
}

function bar(c)
{
	c = 12
}
