BEGIN {
	x = y = "s"
	a = (getline x y)
	print a, x
	a = (getline x + 1)
	print a, x
	a = (getline x - 2)
	print a, x

	cmd = "echo A"
	a = (cmd | getline x y)
	close(cmd)
	print a, x

	cmd = "echo B"
	a = (cmd | getline x + 1)
	close(cmd)
	print a, x

	cmd = "echo C"
	a = (cmd | getline x - 2)
	close(cmd)
	print a, x

	cmd = "echo D"
	a = cmd | getline x
	close(cmd)
	print a, x

	# Concatenation has higher precedence than IO.
	"echo " "date" | getline
	print
}
