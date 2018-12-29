BEGIN {
	cmd = "echo 3"
	y = 7
	cmd | getline x y
	close(cmd)
	print (cmd | getline x y)
}
