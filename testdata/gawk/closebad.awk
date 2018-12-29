BEGIN {
	f = "/no/such/file/1"
	print (getline junk < f)
	print close(f)
	f = "/no/such/file/2"
	print close(f)
}
