# From james@ruari-quinn.demon.co.uk  Thu Jun  5 11:43:58 2003
# Return-Path: <james@ruari-quinn.demon.co.uk>
# Received: from localhost (skeeve [127.0.0.1])
# 	by skeeve.com (8.12.5/8.12.5) with ESMTP id h558eVui012655
# 	for <arnold@localhost>; Thu, 5 Jun 2003 11:43:58 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.9.0)
# 	for arnold@localhost (single-drop); Thu, 05 Jun 2003 11:43:58 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Thu Jun  5 11:43:29 2003)
# X-From_: james@ruari-quinn.demon.co.uk Wed Jun  4 20:09:54 2003
# Received: from smtp1.actcom.net.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id h54H9oY05088 for <arobbins@actcom.co.il>;
# 	Wed, 4 Jun 2003 20:09:52 +0300 (EET DST)  
# 	(rfc931-sender: smtp.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by smtp1.actcom.net.il (8.12.8/8.12.8) with ESMTP id h54HB8To002721
# 	for <arobbins@actcom.co.il>; Wed, 4 Jun 2003 20:11:09 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.7/8.11.6) with ESMTP id h54H9li15411
# 	for <arnold@skeeve.com>; Wed, 4 Jun 2003 13:09:47 -0400
# Received: from monty-python.gnu.org ([199.232.76.173])
# 	by fencepost.gnu.org with esmtp (Exim 4.20)
# 	id 19Nbli-0001kD-BL
# 	for bug-gawk@gnu.org; Wed, 04 Jun 2003 13:09:46 -0400
# Received: from mail by monty-python.gnu.org with spam-scanned (Exim 4.20)
# 	id 19NbZ5-0004V2-71
# 	for bug-gawk@gnu.org; Wed, 04 Jun 2003 12:56:43 -0400
# Received: from cicero.e-mis.co.uk ([212.240.194.162])
# 	by monty-python.gnu.org with esmtp (Exim 4.20)
# 	id 19NbYK-0003c7-AP
# 	for bug-gawk@gnu.org; Wed, 04 Jun 2003 12:55:56 -0400
# Received: from [10.139.58.254] (helo=tacitus)
# 	by cicero.e-mis.co.uk with esmtp (Exim 3.35 #1 (Debian))
# 	id 19NbWO-0007Qv-00
# 	for <bug-gawk@gnu.org>; Wed, 04 Jun 2003 17:53:56 +0100
# Received: from james by tacitus with local (Exim 3.36 #1 (Debian))
# 	id 19NbWO-0000cK-00
# 	for <bug-gawk@gnu.org>; Wed, 04 Jun 2003 17:53:56 +0100
# To: bug-gawk@gnu.org
# Subject: 3.1.0 regression
# Mail-Copies-To: never
# From: James Troup <james@nocrew.org>
# User-Agent: Gnus/5.090017 (Oort Gnus v0.17) Emacs/20.7 (gnu/linux)
# Date: Wed, 04 Jun 2003 17:53:56 +0100
# Message-ID: <874r35wzq3.fsf@nocrew.org>
# MIME-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# Sender: James Troup <james@ruari-quinn.demon.co.uk>
# X-Spam-Status: No, hits=-3.9 required=5.0
# 	tests=EMAIL_ATTRIBUTION,SIGNATURE_SHORT_DENSE,SPAM_PHRASE_00_01,
# 	      USER_AGENT
# 	version=2.41
# X-Spam-Level: 
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: R
# 
# Hi Aharon,
# 
# This bug report comes from the Debian bug tracking system.  You can
# view the full log at:
# 
#  http://bugs.debian.org/188345
# 
# Like my other bug, this is a regression from 3.1.0 and I've reproduced
# this problem with 3.1.2d.
# 
# "Nikita V. Youshchenko" <yoush@cs.msu.su> writes:
# 
# | Package: gawk
# | Version: 1:3.1.2-2
# | Severity: normal
# | Tags: sid
# | 
# | After upgrading gawk from woody to sid, I found one of my scripts not
# | working. I explored this a little and found minimal script to reproduce
# | the problem.
# | 
# | File bug.awk is the following:
# | 
BEGIN  {
  WI_total = 0
}
{
  WI_total++
  {
    split (  $1, sws, "_" )
    a = sws[1]
  }
  print(sws[1])
  print(a)
}
# | 
# | The second print should output the same what first print poutputs, but
# | with gawk 3.1.2-2 it outputs nothing:
# | > echo a_b | gawk -f bug.awk
# | a
# | 
# | >
# | 
# | With gawk from stable I get what expexted:
# | > echo a_b | gawk -f bug.awk
# | a
# | a
# | >
# | 
# | If I remove "WI_total++" line, bug disapperas
# | 
# | -- System Information:
# | Debian Release: 3.0
# | Architecture: i386
# | Kernel: Linux zigzag 2.4.19 16:49:13 MSK 2003 i686
# | Locale: LANG=ru_RU.KOI8-R, LC_CTYPE=ru_RU.KOI8-R
# | 
# | Versions of packages gawk depends on:
# | ii  libc6                         2.3.1-16   GNU C Library: Shared libraries an
# | 
# | -- no debconf information
# 
# -- 
# James
# 
