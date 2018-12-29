# From spcecdt@armory.com  Wed Apr 30 11:08:48 2003
# Return-Path: <spcecdt@armory.com>
# Received: from localhost (skeeve [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h3U7uZWr015489
# 	for <arnold@localhost>; Wed, 30 Apr 2003 11:08:48 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Wed, 30 Apr 2003 11:08:48 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Wed Apr 30 11:05:01 2003)
# X-From_: spcecdt@armory.com Wed Apr 30 04:06:46 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h3U16iv04111 for <arobbins@actcom.co.il>;
# 	Wed, 30 Apr 2003 04:06:45 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h3U16nEv009589
# 	for <arobbins@actcom.co.il>; Wed, 30 Apr 2003 04:06:50 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.7/8.11.6) with ESMTP id h3U16gj29182
# 	for <arnold@skeeve.com>; Tue, 29 Apr 2003 21:06:42 -0400
# Received: from monty-python.gnu.org ([199.232.76.173])
# 	by fencepost.gnu.org with esmtp (Exim 4.10)
# 	id 19Ag3W-00029w-00
# 	for bug-gawk@gnu.org; Tue, 29 Apr 2003 21:06:42 -0400
# Received: from mail by monty-python.gnu.org with spam-scanned (Exim 4.10.13)
# 	id 19Ag1V-0001AN-00
# 	for bug-gawk@gnu.org; Tue, 29 Apr 2003 21:04:39 -0400
# Received: from deepthought.armory.com ([192.122.209.42] helo=armory.com)
# 	by monty-python.gnu.org with smtp (Exim 4.10.13)
# 	id 19Ag1V-0001A3-00
# 	for bug-gawk@gnu.org; Tue, 29 Apr 2003 21:04:37 -0400
# Date: Tue, 29 Apr 2003 18:04:35 -0700
# From: "John H. DuBois III" <spcecdt@armory.com>
# To: bug-gawk@gnu.org
# Subject: gawk 3.1.2a bug
# Message-ID: <20030430010434.GA4278@armory.com>
# Mime-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# Content-Disposition: inline
# User-Agent: Mutt/1.3.28i
# X-Www: http://www.armory.com./~spcecdt/
# Sender: spcecdt@armory.com
# X-Spam-Status: No, hits=-7.2 required=5.0
# 	tests=SIGNATURE_SHORT_DENSE,SPAM_PHRASE_00_01,USER_AGENT,
# 	      USER_AGENT_MUTT
# 	version=2.41
# X-Spam-Level: 
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# gawk-3.1.2a 'BEGIN {foo(bar)};function foo(baz){split("x",baz)}'
# gawk-3.1.2a: cmd. line:1: fatal: split: second argument is not an array
# 
# 	John
# -- 
# John DuBois  spcecdt@armory.com  KC6QKZ/AE  http://www.armory.com/~spcecdt/
# 
BEGIN {
	foo(bar)
}

function foo(baz)
{
	split("x", baz)
}
