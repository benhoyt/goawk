function f(x, y){
	y[1] = x
}
BEGIN {
	f(a, a)
}
