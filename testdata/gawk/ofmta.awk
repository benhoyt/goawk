# Date: Thu, 14 Apr 2011 08:18:55 -0500
# From: j.eh@mchsi.com
# To: arnold@skeeve.com
# Subject: CONVFMT test for the test suite
# Message-ID: <20110414131855.GA1801@apollo>
# 
# Hi,
# 
# Please consider adding this to the test suite. 3.1.8 segfaults
# with this.
# 
# Thanks,
# 
# John
# 
# 
BEGIN {
	i=1.2345
	i=3+i
	a[i]="hi"
	OFMT="%.1f"
	print i                       
	for (x in a) print x, a[x]
	print a[i]
	print "--------"
	CONVFMT=OFMT="%.3f"
	print i                       
	for (x in a) print x, a[x]
	print a[i]
}
