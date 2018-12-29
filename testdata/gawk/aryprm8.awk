BEGIN {
	f(0, a) # nothing
	f(1, a)
}
function f(i, a) {
	if (i == 0) return
	g(a, a)
	pr(a)
}
function g(x, y) {
	h(y, x, y)
}
function h(b, c, d) {
	b[1] = 1
	c[1] = 2 # rewrite
	print b[1], d[1]
	c[2] = 1
	b[2] = 2 # should rewrite
}
function pr(x) {
	print x[1], x[2]
}
