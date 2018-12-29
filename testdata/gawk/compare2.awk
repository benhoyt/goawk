# From beebe@math.utah.edu  Thu Aug  2 15:35:07 2001
# Received: from mail.actcom.co.il [192.114.47.13]
# 	by localhost with POP3 (fetchmail-5.7.4)
# 	for arnold@localhost (single-drop); Thu, 02 Aug 2001 15:35:07 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Thu Aug  2 16:02:36 2001)
# X-From_: beebe@sunshine.math.utah.edu Thu Aug  2 15:41:13 2001
# Received: from lmail.actcom.co.il by actcom.co.il  with ESMTP
# 	(8.9.1a/actcom-0.2) id PAA01349 for <arobbins@actcom.co.il>;
# 	Thu, 2 Aug 2001 15:41:06 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from billohost.com (www.billohost.com [209.196.35.10])
# 	by lmail.actcom.co.il (8.11.2/8.11.2) with ESMTP id f72Cf3I21032
# 	for <arobbins@actcom.co.il>; Thu, 2 Aug 2001 15:41:05 +0300
# Received: from fencepost.gnu.org (we-refuse-to-spy-on-our-users@fencepost.gnu.org [199.232.76.164])
# 	by billohost.com (8.9.3/8.9.3) with ESMTP id IAA28585
# 	for <arnold@skeeve.com>; Thu, 2 Aug 2001 08:34:38 -0400
# Received: from sunshine.math.utah.edu ([128.110.198.2])
# 	by fencepost.gnu.org with esmtp (Exim 3.22 #1 (Debian))
# 	id 15SHjG-00036x-00
# 	for <arnold@gnu.org>; Thu, 02 Aug 2001 08:37:30 -0400
# Received: from suncore.math.utah.edu (IDENT:GsUbUdUYCtFLRE4HvnnvhN4JsjooYcfR@suncore0.math.utah.edu [128.110.198.5])
# 	by sunshine.math.utah.edu (8.9.3/8.9.3) with ESMTP id GAA00190;
# 	Thu, 2 Aug 2001 06:37:04 -0600 (MDT)
# Received: (from beebe@localhost)
# 	by suncore.math.utah.edu (8.9.3/8.9.3) id GAA20469;
# 	Thu, 2 Aug 2001 06:37:03 -0600 (MDT)
# Date: Thu, 2 Aug 2001 06:37:03 -0600 (MDT)
# From: "Nelson H. F. Beebe" <beebe@math.utah.edu>
# To: arnold@gnu.org
# Cc: beebe@math.utah.edu
# X-US-Mail: "Center for Scientific Computing, Department of Mathematics, 322
#         INSCC, University of Utah, 155 S 1400 E RM 233, Salt Lake City, UT
#         84112-0090, USA"
# X-Telephone: +1 801 581 5254
# X-FAX: +1 801 585 1640, +1 801 581 4148
# X-URL: http://www.math.utah.edu/~beebe
# Subject: awk implementations: a bug, or new dark corner?
# Message-ID: <CMM.0.92.0.996755823.beebe@suncore.math.utah.edu>
# Status: RO
# 
# Consider the following program:
# 
# % cat bug.awk
BEGIN {
    split("00/00/00",mdy,"/")
    if ((mdy[1] == 0) && (mdy[2] == 0) && (mdy[3] == 0))
    {
        print "OK: zero strings compare equal to number zero"
        exit(0)
    }
    else
    {
        print "ERROR: zero strings compare unequal to number zero"
        exit(1)
    }
}
# 
# Here are the awk implementation versions (on Sun Solaris 2.7):
# 
# 	% awk -V
# 	awk version 19990416
# 
# 	% mawk -W version
# 	mawk 1.3.3 Nov 1996, Copyright (C) Michael D. Brennan
# 
# 	% nawk -V
# 	awk version 20001115
# 
# 	% gawk --version
# 	GNU Awk 3.1.10
# 	...
# 
# Here's what they say about the test program:
# 
# 	foreach f (awk mawk nawk gawk gawk-*)
# 		echo ======== $f
# 		$f -f ~/bug.awk
# 	end
# 
# 	======== awk
# 	OK: zero strings compare equal to number zero
# 	======== mawk
# 	OK: zero strings compare equal to number zero
# 	======== nawk
# 	OK: zero strings compare equal to number zero
# 	======== gawk
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.0
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.1
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.3
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.4
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.5
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.6
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.60
# 	OK: zero strings compare equal to number zero
# 	======== gawk-3.0.90
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.91
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.92
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.93
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.94
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.95
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.96
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.0.97
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.1.0
# 	ERROR: zero strings compare unequal to number zero
# 	======== gawk-3.1.10
# 	ERROR: zero strings compare unequal to number zero
# 
# Identical results were obtained on Apple Rhapsody, Apple Darwin,
# Compaq/DEC Alpha OSF/1, Intel x86 GNU/Linux, SGI IRIX 6.5, DEC Alpha
# GNU/Linux, and Sun SPARC GNU/Linux, so it definitely is not a C
# compiler problem.
# 
# However, the gray awk book, p. 44, says:
# 
# 	In a comparison expression like:
# 		x == y
# 	if both operands have a numeric type, the comparison is numeric;
# 	otherwise, any numeric operand is converted to a string and the
# 	comparison is made on the string values.
# 
# and the new green gawk book, p. 95, says:
# 
# 	When comparing operands of mixed types, numeric operands are
# 	converted to strings using the value of `CONVFMT'
# 
# This suggests that the OK response in bug.awk is wrong, and the ERROR
# response is correct.  Only recent gawk releases do the right thing,
# and it is awk, mawk, and nawk that have a bug.
# 
# If I change the test program from "00/00/00" to "0/0/0", all versions
# tested produce the OK response.
# 
# Comments?
# 
# After reading the two book excerpts, I changed my code to read
# 
#     if (((0 + mdy[1]) == 0) && ((0 + mdy[2]) == 0) && ((0 + mdy[3]) == 0))
# 
# and output from all implementations now agrees.
# 
# -------------------------------------------------------------------------------
# - Nelson H. F. Beebe                    Tel: +1 801 581 5254                  -
# - Center for Scientific Computing       FAX: +1 801 585 1640, +1 801 581 4148 -
# - University of Utah                    Internet e-mail: beebe@math.utah.edu  -
# - Department of Mathematics, 322 INSCC      beebe@acm.org  beebe@computer.org -
# - 155 S 1400 E RM 233                       beebe@ieee.org                    -
# - Salt Lake City, UT 84112-0090, USA    URL: http://www.math.utah.edu/~beebe  -
# -------------------------------------------------------------------------------
# 
