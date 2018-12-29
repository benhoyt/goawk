# Test program from Paul Eggert, eggert@cs.ucla.edu, Jan. 14, 2005

BEGIN {
	e = "1(e)"
	ex = "3e2(ex)"
	x = "6e5(x)"

	print e+0, x+0
	print 0x
	print 0e+x
	print 0ex
	print 010e2
	print 0e9.3
}

# Expected results:
# 1 600000
# 06e5(x)
# 0600001
# 03e2(ex)
# 1000
# 00.3
