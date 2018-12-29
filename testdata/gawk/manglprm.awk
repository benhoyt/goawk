# From beebe@sunshine.math.utah.edu  Thu Jul 10 00:36:16 2003
# Date: Wed, 9 Jul 2003 06:42:54 -0600 (MDT)
# From: "Nelson H. F. Beebe" <beebe@math.utah.edu>
# To: "Arnold Robbins" <arnold@skeeve.com>
# Cc: beebe@math.utah.edu
# X-US-Mail: "Center for Scientific Computing, Department of Mathematics, 110
#         LCB, University of Utah, 155 S 1400 E RM 233, Salt Lake City, UT
#         84112-0090, USA"
# X-Telephone: +1 801 581 5254
# X-FAX: +1 801 585 1640, +1 801 581 4148
# X-URL: http://www.math.utah.edu/~beebe
# Subject: gawk-3.1.3 (and earlier): reproducible core dump
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# 
# I have a reproducible core dump in gawk-3.1.3, and recent gawk
# versions.
# 
# Consider the following test program,  reduced from a much larger one:
# 
#         % cat gawk-dump.awk

				{ process($0) }

	function out_debug(s)
	{
	     print s
	}

	function process(s,   n,parts)
	{
	    out_debug("Buffer = [" protect(Buffer) "]")
	    Buffer = Buffer s
	    n = split(Buffer,parts,"\n")
	}

	function protect(s)
	{
	    gsub("\n", "\\n", s)
	    return (s)
	}
