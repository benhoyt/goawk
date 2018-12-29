# From djones@zoonami.com  Wed Jun 13 17:46:27 2001
# Received: from mail.actcom.co.il [192.114.47.13]
# 	by localhost with POP3 (fetchmail-5.5.0)
# 	for arnold@localhost (single-drop); Wed, 13 Jun 2001 17:46:27 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Wed Jun 13 17:47:09 2001)
# X-From_: djones@zoonami.com Wed Jun 13 17:45:40 2001
# Received: from lmail.actcom.co.il by actcom.co.il  with ESMTP
# 	(8.9.1a/actcom-0.2) id RAA07057 for <arobbins@actcom.co.il>;
# 	Wed, 13 Jun 2001 17:45:34 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from billohost.com (www.billohost.com [209.196.35.10])
# 	by lmail.actcom.co.il (8.11.2/8.11.2) with ESMTP id f5DEjSO24028
# 	for <arobbins@actcom.co.il>; Wed, 13 Jun 2001 17:45:33 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by billohost.com (8.9.3/8.9.3) with ESMTP id KAA24625
# 	for <arnold@skeeve.com>; Wed, 13 Jun 2001 10:44:43 -0400
# Received: from topcat.zoonami.com ([193.112.141.198])
# 	by fencepost.gnu.org with esmtp (Exim 3.16 #1 (Debian))
# 	id 15ABtZ-0000FQ-00
# 	for <bug-gawk@gnu.org>; Wed, 13 Jun 2001 10:45:21 -0400
# Received: from topcat.zoonami.com (localhost [127.0.0.1])
# 	by topcat.zoonami.com (8.9.3/8.9.3) with ESMTP id OAA28329;
# 	Wed, 13 Jun 2001 14:45:54 GMT
# 	(envelope-from djones@topcat.zoonami.com)
# Message-Id: <200106131445.OAA28329@topcat.zoonami.com>
# To: bug-gawk@gnu.org
# cc: David Jones <drj@pobox.com>
# Subject: 3.1.0 core dumps.  Fiddling with OFMT?
# Date: Wed, 13 Jun 2001 14:45:54 +0000
# From: David Jones <djones@zoonami.com>
# Status: R
# 
# The following program causes gawk to dump core:
# 
# jot 10|./gawk '{OFMT="%."NR"f";print NR}'
# 
# 'jot 10', if you didn't know, produces the numbers 1 to 10 each on its
# own line (ie it's like awk 'BEGIN{for(i=1;i<=10;++i)print i}')
# 
# Here's an example run:
# 
# -- run being
# 1
# 2
# 3
# 4
# gawk: cmd. line:1: (FILENAME=- FNR=5) fatal error: internal error
# Abort trap - core dumped
# -- run end
# 
# Ah.  print NR appears to be not interesting.  The following program also
# has the same problem:
# 
# jot 10|./gawk '{OFMT="%."NR"f"}'
# 
# Cheers,
#  djones
# (version info follows)
# 
# I'm running on FreeBSD 4.1, here's the output of uname -a
# 
# FreeBSD topcat.zoonami.com 4.1-RELEASE FreeBSD 4.1-RELEASE #0: Fri Jul 28 14:30:31 GMT 2000     jkh@ref4.freebsd.org:/usr/src/sys/compile/GENERIC  i386
# 
# And ./gnu --version
# 
# GNU Awk 3.1.0
# Copyright (C) 1989, 1991-2001 Free Software Foundation.
# 
# This program is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 2 of the License, or
# (at your option) any later version.
# 
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
# 
# You should have received a copy of the GNU General Public License
# along with this program; if not, write to the Free Software
# Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
# 
# 
{ OFMT="%."NR"f"; print NR }
