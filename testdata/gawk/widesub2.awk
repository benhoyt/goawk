BEGIN {
	Value = "abc"

	print "Before <" Value "> ", index( Value, "bc" )

	sub( /bc/, "bbc", Value )

	print "After  <" Value ">", index( Value, "bc" )
}
