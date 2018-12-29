BEGIN {
	print "system"
	command = "yes | true"
	system(command)

	print "pipe to command"
	print "hi" | command
	close(command)

	print "pipe from command"
	command | getline x
	close(command)
}
