#From deep@cicada-semi.com  Wed Jan 23 13:15:52 2002
#X-From_: deep@cicada-semi.com Wed Jan 23 01:24:54 2002
#From: "Mandeep Chadha" <deep@cicada-semi.com>
#To: <bug-gawk@gnu.org>
#Subject: gawk version 3.1.0 will not print a ";"
#Date: Tue, 22 Jan 2002 17:23:57 -0600
#Message-ID: <NCBBLGONGLINHCDGFCPNOENHCOAA.deep@cicada-semi.com>
#MIME-Version: 1.0
#Content-Type: text/plain;
#	charset="iso-8859-1"
#Content-Transfer-Encoding: 7bit
#X-Priority: 3 (Normal)
#X-MSMail-Priority: Normal
#X-Mailer: Microsoft Outlook IMO, Build 9.0.2416 (9.0.2911.0)
#Importance: Normal
#X-MimeOLE: Produced By Microsoft MimeOLE V5.50.4807.1700
#
#
#The file "tmp" contains the following lines:
#
#A
#B
#C
#D
#
#and when I run the command:
#
#	gawk '{print "Input = "$_" ; "}' tmp
{print "Input = "$_" ; "}
#
#I get the following output:
#
#Input = A
#Input = B
#Input = C
#Input = D
#
#while I expect the following output:
#
#Input = A ;
#Input = B ;
#Input = C ;
#Input = D ;
#
#Running gawk --version produces the following output:
#
#GNU Awk 3.1.0
#Copyright (C) 1989, 1991-2001 Free Software Foundation.
#
#This program is free software; you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation; either version 2 of the License, or
#(at your option) any later version.
#
#This program is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.
#
#You should have received a copy of the GNU General Public License
#along with this program; if not, write to the Free Software
#Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
#
#I am running this on a i686 machine that is running RedHat 7.2 (out of the box).
#
#Thanks,
#
#Mandeep Chadha
#
#----------------------------------------
#Mandeep Chadha
#Cicada Semiconductor Corp.
#811 Barton Springs Road, Suite 550
#Austin, TX 78704
#Ph:      (512) 327-3500 x111
#E-mail:  deep@cicada-semi.com
#URL:     http://www.cicada-semi.com
#----------------------------------------
