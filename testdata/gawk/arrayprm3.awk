# From spcecdt@armory.com  Fri May  2 13:24:46 2003
# Return-Path: <spcecdt@armory.com>
# Received: from localhost (skeeve [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h42AChum021950
# 	for <arnold@localhost>; Fri, 2 May 2003 13:24:46 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Fri, 02 May 2003 13:24:46 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Fri May  2 13:23:37 2003)
# X-From_: spcecdt@armory.com Fri May  2 00:43:51 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h41Lhm500217 for <arobbins@actcom.co.il>;
# 	Fri, 2 May 2003 00:43:49 +0300 (EET DST)  
# 	(rfc931-sender: lmail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h41LiGcO022817
# 	for <arobbins@actcom.co.il>; Fri, 2 May 2003 00:44:18 +0300
# Received: from armory.com (deepthought.armory.com [192.122.209.42])
# 	by f7.net (8.11.7/8.11.6) with SMTP id h41Lhj106516
# 	for <arnold@skeeve.com>; Thu, 1 May 2003 17:43:46 -0400
# Date: Thu, 1 May 2003 14:43:45 -0700
# From: "John H. DuBois III" <spcecdt@armory.com>
# To: Aharon Robbins <arnold@skeeve.com>
# Subject: Re: gawk 3.1.2a bug
# Message-ID: <20030501214345.GA24615@armory.com>
# References: <200305011738.h41Hcg76017565@localhost.localdomain>
# Mime-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# Content-Disposition: inline
# In-Reply-To: <200305011738.h41Hcg76017565@localhost.localdomain>
# User-Agent: Mutt/1.3.28i
# X-Www: http://www.armory.com./~spcecdt/
# Sender: spcecdt@armory.com
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# On Thu, May 01, 2003 at 08:38:42PM +0300, Aharon Robbins wrote:
# > > That worked, thanks.
# > 
# > Great.  Your report motivated me to find everywhere such additional
# > code ought to be needed.  I think I did so.  --Arnold
# 
# Here's another one (perhaps fixed by your additional work):
# 
BEGIN { foo(a) }
function foo(a) { bar(a); print "" in a }
function bar(a) { a[""]; }
# 
# Prints 1 with gawk-3.1.1; 0 with 3.1.2a.
# 
# 	John
# -- 
# John DuBois  spcecdt@armory.com  KC6QKZ/AE  http://www.armory.com/~spcecdt/
# 
