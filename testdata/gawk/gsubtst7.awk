# From bug-gawk-bounces+arnold=skeeve.com@gnu.org  Tue Jul 12 08:18:24 2011
# Return-Path: <bug-gawk-bounces+arnold=skeeve.com@gnu.org>
# Received: from localhost (localhost [127.0.0.1])
# 	by skeeve.com (8.14.3/8.14.3) with ESMTP id p6C5HArm002260
# 	for <arnold@localhost>; Tue, 12 Jul 2011 08:18:23 +0300
# X-Spam-Checker-Version: SpamAssassin 3.2.5 (2008-06-10) on sls-af11p1
# X-Spam-Level: 
# X-Spam-Status: No, score=-5.5 required=5.0 tests=BAYES_00,DNS_FROM_OPENWHOIS,
# 	RCVD_IN_DNSWL_MED autolearn=ham version=3.2.5
# X-Envelope-From: bug-gawk-bounces+arnold=skeeve.com@gnu.org
# Received: from server1.f7.net [66.148.120.132]
# 	by localhost with IMAP (fetchmail-6.3.11)
# 	for <arnold@localhost> (single-drop); Tue, 12 Jul 2011 08:18:23 +0300 (IDT)
# Received: from lists.gnu.org (lists.gnu.org [140.186.70.17])
# 	by freefriends.org (8.14.4/8.14.4) with ESMTP id p6BIYi4t032040;
# 	Mon, 11 Jul 2011 14:34:48 -0400
# Received: from localhost ([::1]:38787 helo=lists.gnu.org)
# 	by lists.gnu.org with esmtp (Exim 4.71)
# 	(envelope-from <bug-gawk-bounces+arnold=skeeve.com@gnu.org>)
# 	id 1QgLJb-0004tM-Eg
# 	for arnold@skeeve.com; Mon, 11 Jul 2011 14:34:43 -0400
# Received: from eggs.gnu.org ([140.186.70.92]:54022)
# 	by lists.gnu.org with esmtp (Exim 4.71)
# 	(envelope-from <kornet@camk.edu.pl>) id 1QgD0R-0004Vi-HZ
# 	for bug-gawk@gnu.org; Mon, 11 Jul 2011 05:42:24 -0400
# Received: from Debian-exim by eggs.gnu.org with spam-scanned (Exim 4.71)
# 	(envelope-from <kornet@camk.edu.pl>) id 1QgD0Q-0000SE-8u
# 	for bug-gawk@gnu.org; Mon, 11 Jul 2011 05:42:23 -0400
# Received: from moat.camk.edu.pl ([148.81.175.50]:34696)
# 	by eggs.gnu.org with esmtp (Exim 4.71)
# 	(envelope-from <kornet@camk.edu.pl>) id 1QgD0P-0000Px-V3
# 	for bug-gawk@gnu.org; Mon, 11 Jul 2011 05:42:22 -0400
# Received: from localhost (localhost.localdomain [127.0.0.1])
# 	by moat.camk.edu.pl (Postfix) with ESMTP id 72C1D5F004C
# 	for <bug-gawk@gnu.org>; Mon, 11 Jul 2011 11:42:13 +0200 (CEST)
# X-Virus-Scanned: amavisd-new at camk.edu.pl
# Received: from moat.camk.edu.pl ([127.0.0.1])
# 	by localhost (liam.camk.edu.pl [127.0.0.1]) (amavisd-new, port 10024)
# 	with LMTP id oh+-Yw+zHhK6 for <bug-gawk@gnu.org>;
# 	Mon, 11 Jul 2011 11:42:07 +0200 (CEST)
# Received: from gatekeeper.camk.edu.pl (gatekeeper.camk.edu.pl [192.168.1.23])
# 	by moat.camk.edu.pl (Postfix) with ESMTP id 89AA55F0046
# 	for <bug-gawk@gnu.org>; Mon, 11 Jul 2011 11:42:07 +0200 (CEST)
# Received: by gatekeeper.camk.edu.pl (Postfix, from userid 1293)
# 	id 796C8809FB; Mon, 11 Jul 2011 11:42:07 +0200 (CEST)
# Date: Mon, 11 Jul 2011 11:42:07 +0200
# From: Kacper Kornet <draenog@pld-linux.org>
# To: bug-gawk@gnu.org
# Message-ID: <20110711094207.GA2616@camk.edu.pl>
# MIME-Version: 1.0
# Content-Type: text/plain; charset=iso-8859-2
# Content-Disposition: inline
# User-Agent: Mutt/1.5.20 (2009-06-14)
# X-detected-operating-system: by eggs.gnu.org: GNU/Linux 2.6 (newer, 3)
# X-Received-From: 148.81.175.50
# X-Mailman-Approved-At: Mon, 11 Jul 2011 14:34:26 -0400
# Subject: [bug-gawk] Change in behavior of gsub inside loop
# X-BeenThere: bug-gawk@gnu.org
# X-Mailman-Version: 2.1.14
# Precedence: list
# List-Id: "Bug reports and all discussion about gawk." <bug-gawk.gnu.org>
# List-Unsubscribe: <https://lists.gnu.org/mailman/options/bug-gawk>,
# 	<mailto:bug-gawk-request@gnu.org?subject=unsubscribe>
# List-Archive: </archive/html/bug-gawk>
# List-Post: <mailto:bug-gawk@gnu.org>
# List-Help: <mailto:bug-gawk-request@gnu.org?subject=help>
# List-Subscribe: <https://lists.gnu.org/mailman/listinfo/bug-gawk>,
# 	<mailto:bug-gawk-request@gnu.org?subject=subscribe>
# Errors-To: bug-gawk-bounces+arnold=skeeve.com@gnu.org
# Sender: bug-gawk-bounces+arnold=skeeve.com@gnu.org
# Status: R
# 
# Hi,
# 
# I have observed the following changed behavior between gawk-3.8.1 and
# gakw-4.0.0. While in the former 
# 
# echo -ne ' aaa' | gawk '{for (c = 1; c <= NF; c++) {gsub("foo", "bar", $c); print}}'
# 
# prints:
# 
#  aaa
# 
# the gawk-4.0.0 does not preserve the leading spaces and prints:
# 
# aaa
# 
# Best regards,
# -- 
#   Kacper
# 
{for (c = 1; c <= NF; c++) {gsub("foo", "bar", $c); print}}
