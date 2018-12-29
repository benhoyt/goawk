BEGIN {
	split(" 1.234 ", f, "|")	# create a numeric string (strnum) value
	OFMT = "%.1f"
	CONVFMT = "%.2f"

	# Check whether a strnum is displayed the same way before and
	# after force_number is called. Also, should numeric strings
	# be formatted with OFMT and CONVFMT or show the original string value?

	print f[1]	# OFMT
	print (f[1] "")	# CONVFMT

	# force conversion to NUMBER if it has not happened already
	x = f[1]+0

	print f[1]	# OFMT
	print (f[1] "")	# CONVFMT
}
