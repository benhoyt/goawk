BEGIN {
	RS = ""
	FS = "\\"
	$0 = "a\\b"
	print $1
}
