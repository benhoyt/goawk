#From gregfjohnson@yahoo.com  Sun Aug 30 08:36:36 2009
#Return-Path: <gregfjohnson@yahoo.com>
#Received: from aahz (localhost [127.0.0.1])
#	by skeeve.com (8.14.1/8.14.1) with ESMTP id n7U5WoJ2003836
#	for <arnold@localhost>; Sun, 30 Aug 2009 08:36:36 +0300
#X-Spam-Checker-Version: SpamAssassin 3.2.4 (2008-01-01) on server1.f7.net
#X-Spam-Level: 
#X-Spam-Status: No, score=-6.6 required=5.0 tests=BAYES_00,RCVD_IN_DNSWL_MED
#	autolearn=ham version=3.2.4
#X-Envelope-From: gregfjohnson@yahoo.com
#X-Envelope-To: <arnold@skeeve.com>
#Received: from server1.f7.net [64.34.169.74]
#	by aahz with IMAP (fetchmail-6.3.7)
#	for <arnold@localhost> (single-drop); Sun, 30 Aug 2009 08:36:36 +0300 (IDT)
#Received: from fencepost.gnu.org (fencepost.gnu.org [140.186.70.10])
#	by f7.net (8.11.7-20030920/8.11.7) with ESMTP id n7U33m709453
#	for <arnold@skeeve.com>; Sat, 29 Aug 2009 22:03:48 -0500
#Received: from mail.gnu.org ([199.232.76.166]:42095 helo=mx10.gnu.org)
#	by fencepost.gnu.org with esmtp (Exim 4.67)
#	(envelope-from <gregfjohnson@yahoo.com>)
#	id 1Mhai6-0004Qt-3R
#	for bug-gawk@gnu.org; Sat, 29 Aug 2009 23:04:06 -0400
#Received: from Debian-exim by monty-python.gnu.org with spam-scanned (Exim 4.60)
#	(envelope-from <gregfjohnson@yahoo.com>)
#	id 1Mhai5-00062I-EM
#	for bug-gawk@gnu.org; Sat, 29 Aug 2009 23:04:05 -0400
#Received: from web33507.mail.mud.yahoo.com ([68.142.206.156]:28597)
#	by monty-python.gnu.org with smtp (Exim 4.60)
#	(envelope-from <gregfjohnson@yahoo.com>)
#	id 1Mhai5-00061w-2n
#	for bug-gawk@gnu.org; Sat, 29 Aug 2009 23:04:05 -0400
#Received: (qmail 68722 invoked by uid 60001); 30 Aug 2009 03:04:03 -0000
#DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=yahoo.com; s=s1024; t=1251601443; bh=9h2ZOOgxUh+s8Ow5/ZMWUxcviy2L4rpiaNamPAXxhEk=; h=Message-ID:X-YMail-OSG:Received:X-Mailer:Date:From:Subject:To:MIME-Version:Content-Type; b=tWxCQq/aTOT5lhtdPc5hxtXzOjDFmLU6Ao0BSlwbeeBsd9Wl6DU3JCR4gTkoL0aVUOTdjMjgRY7I72yCht+YruDiqZrvtSKvUoAvZAKcPG26RE4jzxUlxQklEHZG9mq9h2gpTIiLYehYDiC0975wukwi/e7ePADfkFwg8eTnT44=
#DomainKey-Signature: a=rsa-sha1; q=dns; c=nofws;
#  s=s1024; d=yahoo.com;
#  h=Message-ID:X-YMail-OSG:Received:X-Mailer:Date:From:Subject:To:MIME-Version:Content-Type;
#  b=LWfhVgxojFG1eYoRrxtrS3YOfH3MTUVTYZle/4utMQEPZQfsmrn6GBwBfThryGqJyZfg38/7JfK9cz/Q3Yt+mf8+xl9/m+Srckc+Xvi42CE0OmoN439vCyhAD8A74XOJsmfKDjJ/+LtioShStUohj1iYDDmRTN4RnnP9X4xnt3c=;
#Message-ID: <410222.68490.qm@web33507.mail.mud.yahoo.com>
#X-YMail-OSG: mfjax.MVM1lI2q5gcl6bChbn6zHgNgj1fByHWJSzB8ZZUmI2QCH6pNwV_IaHxcqecu.VqjKUR6HQhXbziUnX.v5E2nOE61ass9AzqfdVOtKTEAzTPQJ8Z7QB7fq7BMtjn8yohDR6mwOyVTqv3RZh0m1Us7sLit6UmcgeSvJo2rROAmeceq.FBwk2XnEp2_QsljjPHak_WXyvtAK81klDv5qQORWQWqR9q79x7yxORL6fLWwb_x6mZZMSOUaA0p8.ucT453eqT1L8NGkthF.fXmOM3_EYd03zUgr9Sb.zvMvbDC3MCMnVr0JT1uroLmFtVIdTojrFJYFQEDFSB9zT3Ua80ZpGXrjQGx3rZw--
#Received: from [71.165.246.171] by web33507.mail.mud.yahoo.com via HTTP; Sat, 29 Aug 2009 20:04:03 PDT
#X-Mailer: YahooMailClassic/6.1.2 YahooMailWebService/0.7.338.2
#Date: Sat, 29 Aug 2009 20:04:03 -0700 (PDT)
#From: Greg Johnson <gregfjohnson@yahoo.com>
#Subject: bugs in passing uninitialized array to a function
#To: bug-gawk@gnu.org
#MIME-Version: 1.0
#Content-Type: multipart/mixed; boundary="0-1690489838-1251601443=:68490"
#X-detected-operating-system: by monty-python.gnu.org: FreeBSD 6.x (1)
#Status: RO
#
#--0-1690489838-1251601443=:68490
#Content-Type: text/plain; charset=us-ascii
#
#I am using gawk version 3.1.7.
#
#The attached programs illustrate what look to me like two bugs
#in the handling of uninitialized variables to functions that treat
#them as arrays.
#
#Greg Johnson
#
#
#      
#--0-1690489838-1251601443=:68490
#Content-Type: application/octet-stream; name=b1
#Content-Transfer-Encoding: base64
#Content-Disposition: attachment; filename="b1"

# bug?  on uninitialized array, length(a) prints as 3, then the loop
# behaves differently, iterating once.  so, length() behaves differently
# on two calls to the same variable, which was not changed.

function prt1(a, len)
{
    print "length:  " length(a)

    for (i = 1; i <= length(a); i++)
        printf "<" i "," a[i] "> "

    print "\n"
}

BEGIN {
    prt1(zzz)
}

#--0-1690489838-1251601443=:68490
#Content-Type: application/octet-stream; name=b2
#Content-Transfer-Encoding: base64
#Content-Disposition: attachment; filename="b2"

# shouldn't an uninitialized array have length zero?
# length is printed as 1, and the loop iterates once.

function prt(a, len)
{
    len = length(a)
    print "length:  " len

    for (i = 1; i <= len; i++)
        printf "<" i "," a[i] "> "

    print "\n"
}

BEGIN {
    prt(zzz)
}

#--0-1690489838-1251601443=:68490--

