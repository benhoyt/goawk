BEGIN {
	OFMT = "%.8g"
	x = 1
	x += .1
	x = (x "a")
	print x
}
