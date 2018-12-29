# check the precedence of operators:
BEGIN {
	$1 = i = 1
	$+i++
	$- -i++
	print
}
