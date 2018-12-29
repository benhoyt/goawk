# From murata@nips.ac.jp  Tue Aug  6 08:02:14 2002
# Return-Path: <murata@nips.ac.jp>
# Received: from localhost (aahz [127.0.0.1])
# 	by skeeve.com (8.11.2/8.11.2) with ESMTP id g7652Ej01784
# 	for <arnold@localhost>; Tue, 6 Aug 2002 08:02:14 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.7.4)
# 	for arnold@localhost (single-drop); Mon, 05 Aug 2002 22:02:14 -0700 (PDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Tue Aug  6 08:13:06 2002)
# X-From_: murata@nips.ac.jp Tue Aug  6 07:26:32 2002
# Received: from lmail.actcom.co.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id g764QTu27770 for <arobbins@actcom.co.il>;
# 	Tue, 6 Aug 2002 07:26:30 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by lmail.actcom.co.il (8.11.6/8.11.6) with ESMTP id g764QRi04673
# 	for <arobbins@actcom.co.il>; Tue, 6 Aug 2002 07:26:28 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.6/8.11.6) with ESMTP id g764QQ920486
# 	for <arnold@skeeve.com>; Tue, 6 Aug 2002 00:26:26 -0400
# Received: from ccms.nips.ac.jp ([133.48.72.2])
# 	by fencepost.gnu.org with smtp (Exim 3.35 #1 (Debian))
# 	id 17bvvL-00011b-00
# 	for <bug-gawk@gnu.org>; Tue, 06 Aug 2002 00:26:23 -0400
# Received: (from murata@localhost)
# 	by ccms.nips.ac.jp (8.9.3+3.2W/3.7W) id NAA01026;
# 	Tue, 6 Aug 2002 13:26:21 +0900
# Date: Tue, 6 Aug 2002 13:26:21 +0900
# Message-Id: <200208060426.NAA01026@ccms.nips.ac.jp>
# To: bug-gawk@gnu.org
# Cc: murata@nips.ac.jp
# Subject: Bug Report (gawk)
# From: murata@nips.ac.jp (MURATA Yasuhisa)
# Mime-Version: 1.0
# Content-Type: text/plain; charset=US-ASCII
# X-Mailer: mnews [version 1.21PL5] 1999-04/04(Sun)
# 
# Hello, I report a bug.
# 
# 
# == PROGRAM (filename: atest.awk) ==
BEGIN {
  RS=""
}

NR==1 {
  print 1
  RS="\n"
  next
}

NR==2 {
  print 2
  RS=""
  next
}

NR==3 {
  print 3
  RS="\n"
  next
}
# ====
# 
# == DATA (filename: atest.txt) ==
# 1111
# 
# 2222
# 
# ====
# note: last line is "\n".
# 
# 
# == RUN (gawk) ==
# > gawk -f atest.awk atest.txt
# 1
# 2
# (no stop!)
# ====
# 
# == RUN (nawk) ==
# > nawk -f atest.awk atest.txt
# 1
# 2
# 3
# ====
# 
# == VERSION ==
# > gawk --version
# GNU Awk 3.1.1
# Copyright (C) 1989, 1991-2002 Free Software Foundation.
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
# ==
# 
# --
# MURATA Yasuhisa, Technical Staff
# National Institute for Physiological Sciences
# E-mail: murata@nips.ac.jp
