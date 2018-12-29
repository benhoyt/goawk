# Date: Wed, 08 Dec 2004 12:59:42 +0600
# From: Alexander Sashnov <asashnov@sw-soft.com>
# Subject: addon to gawk test suite
# Sender: asashnov@sashnov.plesk.ru
# To: "Arnold D. Robbins" <arnold@skeeve.com>
# Message-id: <lzy8g9xokh.fsf@sashnov.plesk.ru>
# 
# 
# Hello, Arnold.
# 
# I'm hit bug on SuSE 9.1 with awk:
# 
# vsuse91:~ # echo "a:b:c" | awk '{ print $2 }' 'FS=[ :]'
# b
# vsuse91:~ # echo "a:b:c" | awk '{ print $2 }' 'FS=[ :]+'
# awk: cmd. line:2: fatal: Trailing backslash: /[ :]+/
# 
# vsuse91:~ # awk --version
# GNU Awk 3.1.3
# 
# 
# 
# But on my Debian machine all OK:
# 
# asashnov@sashnov:~$ echo "a:b:c" | awk '{ print $2 }' 'FS=[ :]'
# b
# asashnov@sashnov:~$ echo "a:b:c" | awk '{ print $2 }' 'FS=[ :]+'
# b
# asashnov@sashnov:~$ awk --version
# GNU Awk 3.1.4
# 
# 
# Need add test for this sample to gawk test suite for avoid this problems in future.
# -- 
# Alexander Sashnov
# Plesk QA Engineer
# SWsoft, Inc.
# E-mail: asashnov@sw-soft.com
# ICQ UIN: 79404252

{ print $2 }
