# From spcecdt@armory.com  Thu Jun 14 13:24:32 2001
# Received: from mail.actcom.co.il [192.114.47.13]
# 	by localhost with POP3 (fetchmail-5.5.0)
# 	for arnold@localhost (single-drop); Thu, 14 Jun 2001 13:24:32 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Thu Jun 14 13:25:13 2001)
# X-From_: spcecdt@armory.com Thu Jun 14 06:34:47 2001
# Received: from lmail.actcom.co.il by actcom.co.il  with ESMTP
# 	(8.9.1a/actcom-0.2) id GAA29661 for <arobbins@actcom.co.il>;
# 	Thu, 14 Jun 2001 06:34:46 +0300 (EET DST)  
# 	(rfc931-sender: lmail.actcom.co.il [192.114.47.13])
# Received: from billohost.com (www.billohost.com [209.196.35.10])
# 	by lmail.actcom.co.il (8.11.2/8.11.2) with ESMTP id f5E3YiO27337
# 	for <arobbins@actcom.co.il>; Thu, 14 Jun 2001 06:34:45 +0300
# Received: from fencepost.gnu.org (we-refuse-to-spy-on-our-users@fencepost.gnu.org [199.232.76.164])
# 	by billohost.com (8.9.3/8.9.3) with ESMTP id XAA02681
# 	for <arnold@skeeve.com>; Wed, 13 Jun 2001 23:33:57 -0400
# Received: from deepthought.armory.com ([192.122.209.42])
# 	by fencepost.gnu.org with smtp (Exim 3.16 #1 (Debian))
# 	id 15ANu2-00005C-00
# 	for <bug-gawk@gnu.org>; Wed, 13 Jun 2001 23:34:38 -0400
# Date: Wed, 13 Jun 2001 20:32:42 -0700
# From: "John H. DuBois III" <spcecdt@armory.com>
# To: bug-gawk@gnu.org
# Subject: gawk 3.1.0 bug
# Message-ID: <20010613203242.A29975@armory.com>
# Mime-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# X-Mailer: Mutt 1.0.1i
# X-Www: http://www.armory.com./~spcecdt/
# Sender: spcecdt@armory.com
# Status: RO
# 
# Under SCO OpenServer 5.0.6a using gawk 3.1.0 compiled with gcc 2.95.2, this
# program:

    BEGIN {
	f1(Procs,b)
	print "test"
    }

    function f1(Procs,a) {
	# a[""]
	a[""] = "a"	# ADR: Give it a value so can trace it
	f2()
    }

    function f2() {
	# b[""]
	b[""] = "b"	# ADR: Give it a value so can trace it
    }

    # ADR: 1/28/2003: Added this:
    BEGIN { for (i in b) printf("b[\"%s\"] = \"%s\"\n", i, b[i]) }
    # END ADR added.

# gives:
# 
#     gawk: ./gtest:5: fatal error: internal error
# 
# and dumps core.
# 
# gdb gives me this stack backtrace:
# 
# #0  0x80019943 in kill () from /usr/lib/libc.so.1
# #1  0x8003e754 in abort () from /usr/lib/libc.so.1
# #2  0x8062a87 in catchsig (sig=0, code=0) at main.c:947
# #3  0x80053a0c in _sigreturn () from /usr/lib/libc.so.1
# #4  0x80023d36 in cleanfree () from /usr/lib/libc.so.1
# #5  0x80023156 in _real_malloc () from /usr/lib/libc.so.1
# #6  0x80023019 in malloc () from /usr/lib/libc.so.1
# #7  0x8053b95 in do_print (tree=0x0) at builtin.c:1336
# #8  0x806b47c in interpret (tree=0x8084ee4) at eval.c:606
# #9  0x806ad8d in interpret (tree=0x8084f0c) at eval.c:384
# #10 0x806ad21 in interpret (tree=0x8084f5c) at eval.c:367
# #11 0x8061d5b in main (argc=4, argv=0x80478ac) at main.c:506
# 
# 	John
# --
# John DuBois  spcecdt@armory.com.  KC6QKZ/AE  http://www.armory.com./~spcecdt/
# 
