# From spcecdt@armory.com  Tue Apr 15 17:35:01 2003
# Return-Path: <spcecdt@armory.com>
# Received: from localhost (aahz [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h3FEYA6o001541
# 	for <arnold@localhost>; Tue, 15 Apr 2003 17:35:01 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Tue, 15 Apr 2003 17:35:01 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Tue Apr 15 17:38:46 2003)
# X-From_: spcecdt@armory.com Tue Apr 15 11:09:12 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h3F88uC19825 for <arobbins@actcom.co.il>;
# 	Tue, 15 Apr 2003 11:09:04 +0300 (EET DST)  
# 	(rfc931-sender: smtp.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h3F8CgQ7019081
# 	for <arobbins@actcom.co.il>; Tue, 15 Apr 2003 11:12:47 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.7/8.11.6) with ESMTP id h3F88oW23381
# 	for <arnold@skeeve.com>; Tue, 15 Apr 2003 04:08:50 -0400
# Received: from monty-python.gnu.org ([199.232.76.173])
# 	by fencepost.gnu.org with esmtp (Exim 4.10)
# 	id 195LUo-0001cv-00
# 	for bug-gawk@gnu.org; Tue, 15 Apr 2003 04:08:50 -0400
# Received: from mail by monty-python.gnu.org with spam-scanned (Exim 4.10.13)
# 	id 195LUh-0006n0-00
# 	for bug-gawk@gnu.org; Tue, 15 Apr 2003 04:08:44 -0400
# Received: from deepthought.armory.com ([192.122.209.42] helo=armory.com)
# 	by monty-python.gnu.org with smtp (Exim 4.10.13)
# 	id 195LUC-0006JM-00
# 	for bug-gawk@gnu.org; Tue, 15 Apr 2003 04:08:13 -0400
# Date: Tue, 15 Apr 2003 01:08:11 -0700
# From: "John H. DuBois III" <spcecdt@armory.com>
# To: bug-gawk@gnu.org
# Subject: gawk 3.1.2 fatal bug
# Message-ID: <20030415080811.GA14963@armory.com>
# Mime-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# Content-Disposition: inline
# User-Agent: Mutt/1.3.28i
# X-Www: http://www.armory.com./~spcecdt/
# Sender: spcecdt@armory.com
# X-Spam-Status: No, hits=-7.9 required=5.0
# 	tests=SIGNATURE_SHORT_DENSE,SPAM_PHRASE_01_02,USER_AGENT,
# 	      USER_AGENT_MUTT
# 	version=2.41
# X-Spam-Level: 
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# This program:
# 
#     BEGIN { RS = ""; "/bin/echo -n x" | getline }
# 
# fails in exactly the same way under SCO OpenServer 5.0.6a using gawk 3.1.2
# built with gcc 2.95.3 and linux using gawk 3.1.2 built with gcc 3.2.2:
# 
# gawk: gawktest:1: fatal error: internal error
# Abort
# 
# The same program does not fail with gawk 3.1.1.
# 
# 	John
# -- 
# John DuBois  spcecdt@armory.com  KC6QKZ/AE  http://www.armory.com/~spcecdt/
# 
# 
BEGIN {
	RS = ""
	"echo x | tr -d '\\12'" | getline
}
