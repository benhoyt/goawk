BEGIN {
	$0 = "aaa"
	NF = 10
	for (j = 2; j <= NF; ++j) {
		$j = "_"
	}
	print
}
