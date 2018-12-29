function f(a) {
	delete a
	a *= 5
}

BEGIN {
	f(arr)
}
