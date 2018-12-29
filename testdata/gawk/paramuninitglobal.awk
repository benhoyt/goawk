function f(x)
{
	a = 10
	x = 90
	print x
	print a
	a++
	x++
	print x
}

BEGIN {
	f(a)
	print a
}
