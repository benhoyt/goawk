function test(x) {
	print x
	getline
	print x
}

{
	test($0)
}
