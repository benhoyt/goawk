BEGIN {
	# initialize cint arrays
        pos[0] = 0
        posout[0] = 0
        split("00000779770060", f)      # f[1] is a strnum
        pos[f[1]] = 1                   # subscripts must be strings!
        for (x in pos) {
                # if x is a strnum, then the
                # x != 0 test may convert it to an integral NUMBER,
		# and we might lose the unusual string representation
		# if the cint code is not careful to recognize that this is
		# actually a string
                if (x != 0)
                        posout[x] = pos[x]
        }
        # which array element is populated?
        print posout[779770060]
        print posout["00000779770060"]
}
