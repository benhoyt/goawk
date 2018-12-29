BEGIN {
	# choose a field separator that is numeric, so we can test whether
	# force_number properly handles unterminated numeric field strings
	FS = "3"
}

{
	print $1+0
	print $1
}
