# From spcecdt@armory.com  Tue May  6 13:42:34 2003
# Return-Path: <spcecdt@armory.com>
# Received: from localhost (aahz [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h46AgG53003519
# 	for <arnold@localhost>; Tue, 6 May 2003 13:42:34 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Tue, 06 May 2003 13:42:34 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Tue May  6 13:48:46 2003)
# X-From_: spcecdt@armory.com Tue May  6 13:26:09 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h46AQ6520133 for <arobbins@actcom.co.il>;
# 	Tue, 6 May 2003 13:26:07 +0300 (EET DST)  
# 	(rfc931-sender: lmail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h46ARSfl010998
# 	for <arobbins@actcom.co.il>; Tue, 6 May 2003 13:27:31 +0300
# Received: from armory.com (deepthought.armory.com [192.122.209.42])
# 	by f7.net (8.11.7/8.11.6) with SMTP id h46AQ1I18183
# 	for <arnold@skeeve.com>; Tue, 6 May 2003 06:26:01 -0400
# Date: Tue, 6 May 2003 03:25:59 -0700
# From: "John H. DuBois III" <spcecdt@armory.com>
# To: Aharon Robbins <arnold@skeeve.com>
# Subject: Re: gawk 3.1.2b now available
# Message-ID: <20030506102559.GA16105@armory.com>
# References: <200305051157.h45Bv4XO003106@localhost.localdomain>
# Mime-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# Content-Disposition: inline
# In-Reply-To: <200305051157.h45Bv4XO003106@localhost.localdomain>
# User-Agent: Mutt/1.3.28i
# X-Www: http://www.armory.com./~spcecdt/
# Sender: spcecdt@armory.com
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# The patch fixed the previous case, but here's another one - this prints
# <null><control-S>:
# 
# BEGIN {
#     RS = ""
#     "echo 'foo\n\nbaz'" | getline
#     "echo 'foo\n\nbaz'" | getline
#     "echo 'bar\n\nbaz'" | getline
#     print x
# }
# 
# 	John
# -- 
# John DuBois  spcecdt@armory.com  KC6QKZ/AE  http://www.armory.com/~spcecdt/
# 
BEGIN {
    RS = ""
    "echo 'foo\n\nbaz'" | getline ; print
    "echo 'foo\n\nbaz'" | getline ; print
    "echo 'bar\n\nbaz'" | getline ; print
    printf "x = <%s>\n", x
}
