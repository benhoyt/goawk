# From spcecdt@armory.com  Mon May  5 14:37:09 2003
# Return-Path: <spcecdt@armory.com>
# Received: from localhost (skeeve [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h45B1GvT031993
# 	for <arnold@localhost>; Mon, 5 May 2003 14:37:09 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Mon, 05 May 2003 14:37:09 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Mon May  5 14:35:11 2003)
# X-From_: spcecdt@armory.com Mon May  5 12:20:20 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h459KC529186 for <arobbins@actcom.co.il>;
# 	Mon, 5 May 2003 12:20:15 +0300 (EET DST)  
# 	(rfc931-sender: smtp.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h459LMfl025854
# 	for <arobbins@actcom.co.il>; Mon, 5 May 2003 12:21:24 +0300
# Received: from armory.com (deepthought.armory.com [192.122.209.42])
# 	by f7.net (8.11.7/8.11.6) with SMTP id h459K9I26841
# 	for <arnold@skeeve.com>; Mon, 5 May 2003 05:20:09 -0400
# Date: Mon, 5 May 2003 02:20:08 -0700
# From: "John H. DuBois III" <spcecdt@armory.com>
# To: Aharon Robbins <arnold@skeeve.com>
# Subject: Re: gawk 3.1.2b now available
# Message-ID: <20030505092008.GA15970@armory.com>
# References: <200305041149.h44BnLcm005484@localhost.localdomain>
# Mime-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# Content-Disposition: inline
# In-Reply-To: <200305041149.h44BnLcm005484@localhost.localdomain>
# User-Agent: Mutt/1.3.28i
# X-Www: http://www.armory.com./~spcecdt/
# Sender: spcecdt@armory.com
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# This is a curious one:
# 
# gawk-3.1.2b 'BEGIN {
#     while (("echo" | getline) == 1)
# 	;
#     RS = ""
#     "echo \"a\n\nb\"" | getline y
#     print x
# }' | hd
# 
# The output is:
# 
# 0000    00 13 0a                                           ...
# 0003
# 
# (the uninitialized variable 'x' is somehow getting the value <null><control-S>)
# 
# 	John
# -- 
# John DuBois  spcecdt@armory.com  KC6QKZ/AE  http://www.armory.com/~spcecdt/
# 
BEGIN {
    while (("echo" | getline) == 1)
	;
    RS = ""
    "echo \"a\n\nb\"" | getline y
    printf "y = <%s>\n", y	# ADR
    printf "x = <%s>\n", x	# ADR
}
