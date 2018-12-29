# From jose@monkey.org  Thu Jun  5 11:48:35 2003
# Return-Path: <jose@monkey.org>
# Received: from localhost (skeeve [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h558eVvA012655
# 	for <arnold@localhost>; Thu, 5 Jun 2003 11:48:35 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Thu, 05 Jun 2003 11:48:35 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Thu Jun  5 11:47:59 2003)
# X-From_: jose@monkey.org Thu Jun  5 07:14:45 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h554EdY08108 for <arobbins@actcom.co.il>;
# 	Thu, 5 Jun 2003 07:14:41 +0300 (EET DST)  
# 	(rfc931-sender: smtp.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h554G3To008304
# 	for <arobbins@actcom.co.il>; Thu, 5 Jun 2003 07:16:05 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.7/8.11.6) with ESMTP id h554Ean08172
# 	for <arnold@skeeve.com>; Thu, 5 Jun 2003 00:14:36 -0400
# Received: from monty-python.gnu.org ([199.232.76.173])
# 	by fencepost.gnu.org with esmtp (Exim 4.20)
# 	id 19Nm96-0001xE-1i
# 	for arnold@gnu.ai.mit.edu; Thu, 05 Jun 2003 00:14:36 -0400
# Received: from mail by monty-python.gnu.org with spam-scanned (Exim 4.20)
# 	id 19Nm8x-0005ge-Dz
# 	for arnold@gnu.ai.mit.edu; Thu, 05 Jun 2003 00:14:28 -0400
# Received: from naughty.monkey.org ([66.93.9.164])
# 	by monty-python.gnu.org with esmtp (Exim 4.20)
# 	id 19Nm8w-0005VM-Ko
# 	for arnold@gnu.ai.mit.edu; Thu, 05 Jun 2003 00:14:26 -0400
# Received: by naughty.monkey.org (Postfix, from userid 1203)
# 	id C15511BA97B; Thu,  5 Jun 2003 00:14:19 -0400 (EDT)
# Received: from localhost (localhost [127.0.0.1])
# 	by naughty.monkey.org (Postfix) with ESMTP
# 	id BF9821BA969; Thu,  5 Jun 2003 00:14:19 -0400 (EDT)
# Date: Thu, 5 Jun 2003 00:14:19 -0400 (EDT)
# From: Jose Nazario <jose@monkey.org>
# To: bug-gnu-utils@prep.ai.mit.edu, arnold@gnu.ai.mit.edu,
#    netbsd-bugs@netbsd.org
# Subject: bug in gawk/gsub() (not present in nawk)
# Message-ID: <Pine.BSO.4.51.0306050007160.31577@naughty.monkey.org>
# MIME-Version: 1.0
# Content-Type: TEXT/PLAIN; charset=US-ASCII
# X-Spam-Status: No, hits=-1.2 required=5.0
# 	tests=SPAM_PHRASE_00_01,USER_AGENT_PINE
# 	version=2.41
# X-Spam-Level: 
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: R
# 
# while playing with some tools in data massaging, i had to migrate from an
# openbsd/nawk system to a netbsd/gawk system. i found the folllowing
# behavior, which seems to be a bug.
# 
# the following gsub() pattern has a strange effect under gawk which is not
# visible in nawk (at least as compiled on openbsd). the intention is to
# take a string like "This Is a Title: My Title?" and turn it into a
# normalized string: "ThisIsaTitleMyTitle". to do this, i wrote the
# following gross gsub line in an awk script:
# 
# 	gsub(/[\ \"-\/\\:;\[\]\@\?\.\,\$]/, "", $2)
# 	print $2
# 
# in gawk, as found in netbsd-macppc/1.5.2, this will drop the first letter
# of every word. the resulting string will be "hissitleyitle", while in nawk
# as built on openbsd-3.3 this will get it correct.
# 
# any insights? the inconsistency with this relatively naive pattern seems a
# bit odd. (i would up installing nawk built from openbsd sources.)
# 
# thanks. sorry i didn't send a better bug report, netbsd folks, i'm not
# much of a netbsd user, and i dont have send-pr set up. yes, this is a
# slightly older version of netbsd and gawk:
# 
# $ uname -a
# NetBSD entropy 1.5.2 NetBSD 1.5.2 (GENERIC) #0: Sun Feb 10 02:00:04 EST
# 2002     jose@entropy:/usr/src/sys/arch/macppc/compile/GENERIC macppc
# $ awk --version
# GNU Awk 3.0.3
# Copyright (C) 1989, 1991-1997 Free Software Foundation.
# 
# 
# 
# thanks.
# 
# ___________________________
# jose nazario, ph.d.			jose@monkey.org
# 					http://monkey.org/~jose/
# 
# 
{
	gsub(/[\ \"-\/\\:;\[\]\@\?\.\,\$]/, "")
 	print
}
