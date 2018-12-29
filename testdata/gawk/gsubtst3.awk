# From laura_fairhead@talk21.com  Fri May 10 11:24:41 2002
# Return-Path: <laura_fairhead@talk21.com>
# Received: from localhost (aahz [127.0.0.1])
# 	by skeeve.com (8.11.2/8.11.2) with ESMTP id g4A8OdU01822
# 	for <arnold@localhost>; Fri, 10 May 2002 11:24:40 +0300
# Received: from actcom.co.il [192.114.47.1]
# 	by localhost with POP3 (fetchmail-5.7.4)
# 	for arnold@localhost (single-drop); Fri, 10 May 2002 11:24:40 +0300 (IDT)
# Received: by actcom.co.il (mbox arobbins)
#  (with Cubic Circle's cucipop (v1.31 1998/05/13) Fri May 10 11:30:42 2002)
# X-From_: laura_fairhead@talk21.com Fri May 10 05:39:57 2002
# Received: from lmail.actcom.co.il by actcom.co.il  with ESMTP
# 	(8.11.6/actcom-0.2) id g4A2dpw26380 for <arobbins@actcom.co.il>;
# 	Fri, 10 May 2002 05:39:52 +0300 (EET DST)  
# 	(rfc931-sender: mail.actcom.co.il [192.114.47.13])
# Received: from f7.net (consort.superb.net [209.61.216.22])
# 	by lmail.actcom.co.il (8.11.6/8.11.6) with ESMTP id g4A2dxl10851
# 	for <arobbins@actcom.co.il>; Fri, 10 May 2002 05:39:59 +0300
# Received: from fencepost.gnu.org (fencepost.gnu.org [199.232.76.164])
# 	by f7.net (8.11.6/8.11.6) with ESMTP id g4A2dwN11097
# 	for <arnold@skeeve.com>; Thu, 9 May 2002 22:39:58 -0400
# Received: from [194.73.242.6] (helo=wmpmta04-app.mail-store.com)
# 	by fencepost.gnu.org with smtp (Exim 3.34 #1 (Debian))
# 	id 1760K4-0001QX-00
# 	for <bug-gawk@gnu.org>; Thu, 09 May 2002 22:39:56 -0400
# Received: from wmpmtavirtual ([10.216.84.15])
#           by wmpmta04-app.mail-store.com
#           (InterMail vM.5.01.02.00 201-253-122-103-101-20001108) with SMTP
#           id <20020510023921.EEW24107.wmpmta04-app.mail-store.com@wmpmtavirtual>
#           for <bug-gawk@gnu.org>; Fri, 10 May 2002 03:39:21 +0100
# Received: from 213.1.102.243 by t21web05-lrs ([10.216.84.15]); Fri, 10 May 02 03:38:42 GMT+01:00
# X-Mailer: talk21 v1.24 - http://talk21.btopenworld.com
# From: laura_fairhead@talk21.com
# To: bug-gawk@gnu.org
# X-Talk21Ref: none
# Date: Fri, 10 May 2002 03:38:42 GMT+01:00
# Subject: bug in gawk 3.1.0 regex code
# Mime-Version: 1.0
# Content-type: multipart/mixed; boundary="--GgOuLpDpIyE--1020998322088--"
# Message-Id: <20020510023921.EEW24107.wmpmta04-app.mail-store.com@wmpmtavirtual>
# X-SpamBouncer: 1.4 (10/07/01)
# X-SBClass: OK
# Status: RO
# 
# Multipart Message Boundary - attachment/bodypart follows:
# 
# 
# ----GgOuLpDpIyE--1020998322088--
# Content-Type: text/plain
# Content-Transfer-Encoding: 7bit
# 
# 
# I believe I've just found a bug in gawk3.1.0 implementation of
# extended regular expressions. It seems to be down to the alternation
# operator; when using an end anchor '$' as a subexpression in an
# alternation and the entire matched RE is a nul-string it fails
# to match the end of string, for example;
# 
# gsub(/$|2/,"x")
# print
# 
# input           = 12345
# expected output = 1x345x
# actual output   = 1x345
# 
# The start anchor '^' always works as expected;
# 
# gsub(/^|2/,"x")
# print
# 
# input           = 12345
# expected output = x1x345
# actual output   = x1x345
# 
# This was with POSIX compliance enabled althought that doesn't
# effect the result.
# 
# I checked on gawk3.0.6 and got exactly the same results however
# gawk2.15.6 gives the expected results.
# 
# All the follow platforms produced the same results;
# 
# gawk3.0.6 / Win98 / i386
# gawk3.1.0 / Win98 / i386
# gawk3.0.5 / Linux2.2.16 / i386
# 
# Complete test results were as follows;
# 
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# regex               input     expected  actual    bug?      
# -------------------------------------------------------------
# (^)                 12345     x12345    x12345              
# ($)                 12345     12345x    12345x              
# (^)|($)             12345     x12345x   x12345x             
# ($)|(^)             12345     x12345x   x12345x             
# 2                   12345     1x345     1x345               
# (^)|2               12345     x1x345    x1x345              
# 2|(^)               12345     x1x345    x1x345              
# ($)|2               12345     1x345x    1x345     **BUG**   
# 2|($)               12345     1x345x    1x345     **BUG**   
# (2)|(^)             12345     x1x345    x1x345              
# (^)|(2)             12345     x1x345    x1x345              
# (2)|($)             12345     1x345x    1x345     **BUG**   
# ($)|(2)             12345     1x345x    1x345     **BUG**   
# ((2)|(^)).          12345     xx45      xx45                
# ((^)|(2)).          12345     xx45      xx45                
# .((2)|($))          12345     x34x      x34x                
# .(($)|(2))          12345     x34x      x34x                
# (^)|6               12345     x12345    x12345              
# 6|(^)               12345     x12345    x12345              
# ($)|6               12345     12345x    12345x              
# 6|($)               12345     12345x    12345x              
# 2|6|(^)             12345     x1x345    x1x345              
# 2|(^)|6             12345     x1x345    x1x345              
# 6|2|(^)             12345     x1x345    x1x345              
# 6|(^)|2             12345     x1x345    x1x345              
# (^)|6|2             12345     x1x345    x1x345              
# (^)|2|6             12345     x1x345    x1x345              
# 2|6|($)             12345     1x345x    1x345     **BUG**   
# 2|($)|6             12345     1x345x    1x345     **BUG**   
# 6|2|($)             12345     1x345x    1x345     **BUG**   
# 6|($)|2             12345     1x345x    1x345     **BUG**   
# ($)|6|2             12345     1x345x    1x345     **BUG**   
# ($)|2|6             12345     1x345x    1x345     **BUG**   
# 2|4|(^)             12345     x1x3x5    x1x3x5              
# 2|(^)|4             12345     x1x3x5    x1x3x5              
# 4|2|(^)             12345     x1x3x5    x1x3x5              
# 4|(^)|2             12345     x1x3x5    x1x3x5              
# (^)|4|2             12345     x1x3x5    x1x3x5              
# (^)|2|4             12345     x1x3x5    x1x3x5              
# 2|4|($)             12345     1x3x5x    1x3x5     **BUG**   
# 2|($)|4             12345     1x3x5x    1x3x5     **BUG**   
# 4|2|($)             12345     1x3x5x    1x3x5     **BUG**   
# 4|($)|2             12345     1x3x5x    1x3x5     **BUG**   
# ($)|4|2             12345     1x3x5x    1x3x5     **BUG**   
# ($)|2|4             12345     1x3x5x    1x3x5     **BUG**   
# x{0}((2)|(^))       12345     x1x345    x1x345              
# x{0}((^)|(2))       12345     x1x345    x1x345              
# x{0}((2)|($))       12345     1x345x    1x345     **BUG**   
# x{0}(($)|(2))       12345     1x345x    1x345     **BUG**   
# x*((2)|(^))         12345     x1x345    x1x345              
# x*((^)|(2))         12345     x1x345    x1x345              
# x*((2)|($))         12345     1x345x    1x345     **BUG**   
# x*(($)|(2))         12345     1x345x    1x345     **BUG**   
# x{0}^               12345     x12345    x12345              
# x{0}$               12345     12345x    12345x              
# (x{0}^)|2           12345     x1x345    x1x345              
# (x{0}$)|2           12345     1x345x    1x345     **BUG**   
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# 
# 
# Here's the test program I used, a few of the cases use ERE {n[,[m]]}
# operators so need '-W posix', (although the same results minus
# those tests came out without POSIX compliance enabled)
# 
# [ Invocation was 'gawk -W posix -f tregex.awk' ]
# 
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# tregex.awk
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
BEGIN{
print _=sprintf("%-20s%-10s%-10s%-10s%-10s\n","regex","input","expected","actual","bug?")
OFS="-"
$(length(_)+1)=""
print $0

#while(getline <ARGV[1]) # ADR: was testre.dat
while(getline) # ADR: use stdin so can automate generation of test
{
RE=$1;IN=$2;OUT=$3
$0=IN
gsub(RE,"x")
printf "%-20s%-10s%-10s%-10s%-10s\n",RE,IN,OUT,$0,$0==OUT?"":"**BUG**"
}
}
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# 
# This is the test data file used;
# 
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# testre.dat
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# (^)             12345           x12345
# ($)             12345           12345x
# (^)|($)         12345           x12345x
# ($)|(^)         12345           x12345x
# 2               12345           1x345
# (^)|2           12345           x1x345
# 2|(^)           12345           x1x345
# ($)|2           12345           1x345x
# 2|($)           12345           1x345x
# (2)|(^)         12345           x1x345
# (^)|(2)         12345           x1x345
# (2)|($)         12345           1x345x
# ($)|(2)         12345           1x345x
# ((2)|(^)).      12345           xx45
# ((^)|(2)).      12345           xx45
# .((2)|($))      12345           x34x
# .(($)|(2))      12345           x34x
# (^)|6           12345           x12345
# 6|(^)           12345           x12345
# ($)|6           12345           12345x
# 6|($)           12345           12345x
# 2|6|(^)         12345           x1x345
# 2|(^)|6         12345           x1x345
# 6|2|(^)         12345           x1x345
# 6|(^)|2         12345           x1x345
# (^)|6|2         12345           x1x345
# (^)|2|6         12345           x1x345
# 2|6|($)         12345           1x345x
# 2|($)|6         12345           1x345x
# 6|2|($)         12345           1x345x
# 6|($)|2         12345           1x345x
# ($)|6|2         12345           1x345x
# ($)|2|6         12345           1x345x
# 2|4|(^)         12345           x1x3x5
# 2|(^)|4         12345           x1x3x5
# 4|2|(^)         12345           x1x3x5
# 4|(^)|2         12345           x1x3x5
# (^)|4|2         12345           x1x3x5
# (^)|2|4         12345           x1x3x5
# 2|4|($)         12345           1x3x5x
# 2|($)|4         12345           1x3x5x
# 4|2|($)         12345           1x3x5x
# 4|($)|2         12345           1x3x5x
# ($)|4|2         12345           1x3x5x
# ($)|2|4         12345           1x3x5x
# x{0}((2)|(^))   12345           x1x345
# x{0}((^)|(2))   12345           x1x345
# x{0}((2)|($))   12345           1x345x
# x{0}(($)|(2))   12345           1x345x
# x*((2)|(^))     12345           x1x345
# x*((^)|(2))     12345           x1x345
# x*((2)|($))     12345           1x345x
# x*(($)|(2))     12345           1x345x
# x{0}^           12345           x12345
# x{0}$           12345           12345x
# (x{0}^)|2       12345           x1x345
# (x{0}$)|2       12345           1x345x
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# 
# I've attached a full copy of this e-mail in ZIP format
# in case of e-mail transport errors corrupting the data.
# 
# I've posted the same bug report to gnu.utils.bug and
# it's being discussed in this thread on comp.lang.awk;
# 
# From: laura@madonnaweb.com (laura fairhead)
# Newsgroups: comp.lang.awk
# Subject: bug in gawk3.1.0 regex code
# Date: Wed, 08 May 2002 23:31:40 GMT
# Message-ID: <3cd9b0f7.29675926@NEWS.CIS.DFN.DE>
# 
# 
# byefrom
# 
# Laura Fairhead
# 
# 
# 
# 
# --------------------
# talk21 your FREE portable and private address on the net at http://www.talk21.com
# ----GgOuLpDpIyE--1020998322088--
# Content-Type: : application/zip;; Name="COPY.ZIP"
# Content-Transfer-Encoding: base64
# Content-Disposition: attachment; filename="COPY.ZIP"
# 
# UEsDBBQAAAAIALoaqiyj8d/bjwMAAKsaAAADAAAARklMrVjfa+JAEH4P5H8ISwrRU9EYfbheKBR6
# xRcLvevbYbFtzsqJlBrpQr3722+zMWZ31pk1MaG0Q/m+nR87O9kvruM6/5p4XOc9WSTc05/l
# +m2bSivhb8lzmrx43vw53c5X2f+etourHOc63XMe1wlmLQ8+g3AYjaTFD2ZplY9g+xRbWly3
# NPastYMrQN9cs4DvHYz+dHbomY8SOTctGDlcQfXND1Uz6cK3EXcVdpY37ltSuB55u339cNtu
# F76NPTudHYR0zS2RZ/sd1maHVLdYI/cp31b2PvFW72jkvIi2tLTI94nXY/eCfeZK8Ap7GO1b
# u7QAO8+8FjsLfFx7OowtfW6dLYRv22wZ031uYYc7M/aK5xvEfjp7vDPnQxW2OZuqndDxWeyw
# dt6y5rXPt5xrqG8bW9a8tm8ZN1q1UyYTXvNT2HjN7VWLLL3GR7pl9nlUkx1Z+5xm2/qcYsu4
# z2KHtfOWNad6jR92jGN9jvm2sSNbn1vYlj4n2TLus9h4zW1s/tn/e3iHV55MOXumvUarsvVX
# +OknNGfrr/AK7DbMulLkbZh1VTa8uFSLHF5cqlVt5tW9eWRsH2VbVY10rp+TCu9Q6Rxj2/Ju
# SJE2KG5TqW57848/jS15fXM7mX66ztv7cp16j/FGGr8DdtEN+5uL7sD49WvNOkwGIv5KaS3+
# FsJamLmyFkYmrFnLde6+/4hZl7mOH6yS9SJ9DR5bXwatmLHCrd/PivTxulwlwSJJV8t14n1j
# abIRCfde5mm2iojx/ib2B5eTaeyHl3cPP2N/KNbsx5Op6yw226fg/qbDeIbNc/DoHAR6Mu2I
# dTp+X/zEsTCvGPvK9j0govsrfxqqdJN9cKhMY0vilwdPOebmRwqIy4+x+Tni+Hrc/PKAAnGZ
# 7pXH2fyaYK6X4+B9CcPBt/RRt9z8FoDhoOpH/QJ9j+KAkkf9As2O4oA6N/xy6RWo8OMoqLYN
# 1DDipqo+joIqEGtQqDWJRibXK9oO6igMB1Uu2XeKZwwHlSuO0zue6idVGVE4VQPheeiVIc8F
# sV6Bg6oRx+knkup3Kl8VR+Vb5qGru2N14SNTx2E4qNhwnH1/+chUYRROvfvjeejK6khdeLm/
# +HoFDqolHGfdX17sG5WviqPyLXBQ1WB9D/ULjSvHH9ZXUJOgOKA+UL9AZ1A4dThTftXxTOWh
# qgRs7kI9gF4gwM0fnVfgjo/F19A96T9QSwECFAAUAAAACAC6Gqoso/Hf248DAACrGgAAAwAA
# AAAAAAABACAAAAAAAAAARklMUEsFBgAAAAABAAEAMQAAALADAAAAAA==
# ----GgOuLpDpIyE--1020998322088----
# 
# 
# 
