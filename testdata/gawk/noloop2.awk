# From jhart@avcnet.bates.edu  Sun Oct  6 16:05:21 2002
# Return-Path: <jhart@avcnet.bates.edu>
# Received: from localhost (skeeve [127.0.0.1])
# 	by skeeve.com (8.11.6/8.11.6) with ESMTP id g96D5Jf28053
# 	for <arnold@localhost>; Sun, 6 Oct 2002 16:05:21 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Sun, 06 Oct 2002 16:05:21 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Sun Oct  6 16:06:39 2002)
# X-From_: jhart@avcnet.bates.edu Sun Oct  6 15:31:59 2002
# Received: from lmail.actcom.co.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id g96CVrS27315 for <arobbins@actcom.co.il>;
# 	Sun, 6 Oct 2002 15:31:54 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by lmail.actcom.co.il (8.11.6/8.11.6) with ESMTP id g96CVqY01629
# 	for <arobbins@actcom.co.il>; Sun, 6 Oct 2002 15:31:52 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.6/8.11.6) with ESMTP id g96CVp418974
# 	for <arnold@skeeve.com>; Sun, 6 Oct 2002 08:31:51 -0400
# Received: from monty-python.gnu.org ([199.232.76.173])
# 	by fencepost.gnu.org with esmtp (Exim 4.10)
# 	id 17yAZa-00055o-00
# 	for bug-gawk@gnu.org; Sun, 06 Oct 2002 08:31:50 -0400
# Received: from mail by monty-python.gnu.org with spam-scanned (Exim 4.10)
# 	id 17yAZE-0007eB-00
# 	for bug-gawk@gnu.org; Sun, 06 Oct 2002 08:31:29 -0400
# Received: from avcnet.bates.edu ([134.181.128.62])
# 	by monty-python.gnu.org with esmtp (Exim 4.10)
# 	id 17yAZ9-0007X3-00
# 	for bug-gawk@gnu.org; Sun, 06 Oct 2002 08:31:23 -0400
# Received: from a5514a.bates.edu (www.bates.edu [134.181.128.62])
# 	by avcnet.bates.edu (8.9.3/8.9.3) with ESMTP id IAA05400
# 	for <bug-gawk@gnu.org>; Sun, 6 Oct 2002 08:31:20 -0400
# Date: Sun, 6 Oct 2002 08:36:54 -0400
# Mime-Version: 1.0 (Apple Message framework v482)
# Content-Type: text/plain; charset=US-ASCII; format=flowed
# Subject: Infinite loop in sub/gsub
# From: jhart@avcnet.bates.edu
# To: bug-gawk@gnu.org
# Content-Transfer-Encoding: 7bit
# Message-Id: <4BC4A4F0-D928-11D6-8E78-00039384A9CC@mail.avcnet.org>
# X-Mailer: Apple Mail (2.482)
# X-Spam-Status: No, hits=0.3 required=5.0
# 	tests=NO_REAL_NAME,SPAM_PHRASE_00_01,USER_AGENT_APPLEMAIL
# 	version=2.41
# X-Spam-Level: 
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# This command line:
# 
# echo "''Italics with an apostrophe'' embedded''"|gawk -f test.awk
# 
# where test.awk contains this instruction:
# 
/''/  { sub(/''(.?[^']+)*''/, "<em>&</em>"); }
# 
# puts gawk 3.11 into an infinite loop. Whereas, this command works:
# 
# echo "''Italics with an apostrophe' embedded''"|gawk -f test.awk
# 
# 
# 
# Platform: Mac OS X 10.1.5/Darwin Kernel Version 5.5: Thu May 30 14:51:26 
# PDT 2002; root:xnu/xnu-201.42.3.obj~1/RELEASE_PPC
# 
# 
