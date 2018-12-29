function f(s, x) {
	x = 1
	s = 3
	s = s x
	print s
}

BEGIN { for (i = 1; i <=12; i++) f() }
