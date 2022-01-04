BEGIN { RS = "" }

{
	print NF, "<" $0 ":" RT ">"
	for (i = 1; i <= NF; i++)
		print i, "[" $i "]"
}
