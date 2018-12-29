# From beebe@math.utah.edu  Sat May 17 21:31:27 2003
# Return-Path: <beebe@math.utah.edu>
# Received: from localhost (aahz [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h4HIQmCw001380
# 	for <arnold@localhost>; Sat, 17 May 2003 21:31:27 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Sat, 17 May 2003 21:31:27 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Sat May 17 21:34:07 2003)
# X-From_: beebe@sunshine.math.utah.edu Fri May 16 20:38:45 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h4GHcd226764 for <arobbins@actcom.co.il>;
# 	Fri, 16 May 2003 20:38:40 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h4GHgBc2023067
# 	for <arobbins@actcom.co.il>; Fri, 16 May 2003 20:42:13 +0300
# Received: from sunshine.math.utah.edu (sunshine.math.utah.edu [128.110.198.2])
# 	by f7.net (8.11.7/8.11.6) with ESMTP id h4GHcbf09202
# 	for <arnold@skeeve.com>; Fri, 16 May 2003 13:38:37 -0400
# Received: from suncore.math.utah.edu (IDENT:r8KQWmkF4jVMLBhxpojXGNCAnBZB38ET@suncore.math.utah.edu [128.110.198.5])
# 	by sunshine.math.utah.edu (8.9.3p2/8.9.3) with ESMTP id LAA09111;
# 	Fri, 16 May 2003 11:38:34 -0600 (MDT)
# Received: (from beebe@localhost)
# 	by suncore.math.utah.edu (8.9.3p2/8.9.3) id LAA01743;
# 	Fri, 16 May 2003 11:38:34 -0600 (MDT)
# Date: Fri, 16 May 2003 11:38:34 -0600 (MDT)
# From: "Nelson H. F. Beebe" <beebe@math.utah.edu>
# To: "Arnold Robbins" <arnold@skeeve.com>
# Cc: beebe@math.utah.edu
# X-US-Mail: "Center for Scientific Computing, Department of Mathematics, 110
#         LCB, University of Utah, 155 S 1400 E RM 233, Salt Lake City, UT
#         84112-0090, USA"
# X-Telephone: +1 801 581 5254
# X-FAX: +1 801 585 1640, +1 801 581 4148
# X-URL: http://www.math.utah.edu/~beebe
# Subject: gawk-3.1.2[ab]: bug in delete
# Message-ID: <CMM.0.92.0.1053106714.beebe@suncore.math.utah.edu>
# 
# I discovered yesterday that one of my tools got broken by the upgrade
# to gawk-3.1.2a and gawk-3.1.2b.  For now, I've temporarily reset
# /usr/local/bin/gawk on the Sun Solaris and Intel GNU/Linux systems
# back to be gawk-3.1.2.
# 
# This morning, I isolated the problem to the following small test case:
# 
# 	% cat bug.awk
	BEGIN {
	    clear_array(table)
	    foo(table)
	    for (key in table)
		print key, table[k]
	    clear_array(table)
	    exit(0)
	}

	function clear_array(array, key)
	{
	    for (key in array)
		delete array[key]
	}

	function foo(a)
	{
	    a[1] = "one"
	    a[2] = "two"
	}
# 
# With nawk, mawk, and also gawk-3.1.2 or earlier, I get this:
# 
# 	% mawk -f bug.awk
# 	1
# 	2
# 
# However, with the two most recent gawk releases, I get:
# 
# 	% gawk-3.1.2b -f bug.awk
# 	gawk-3.1.2b: bug.awk:12: fatal: delete: illegal use of variable `table' as
# 	array
# 
# If the first clear_array() statement is commented out, it runs.
# However, the problem is that in a large program, it may not be easy to
# identify places where it is safe to invoke delete, so I believe the
# old behavior is more desirable.
# 
# -------------------------------------------------------------------------------
# - Nelson H. F. Beebe                    Tel: +1 801 581 5254                  -
# - Center for Scientific Computing       FAX: +1 801 581 4148                  -
# - University of Utah                    Internet e-mail: beebe@math.utah.edu  -
# - Department of Mathematics, 110 LCB        beebe@acm.org  beebe@computer.org -
# - 155 S 1400 E RM 233                       beebe@ieee.org                    -
# - Salt Lake City, UT 84112-0090, USA    URL: http://www.math.utah.edu/~beebe  -
# -------------------------------------------------------------------------------
# 
