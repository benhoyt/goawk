#From arnold  Thu May  9 17:27:03 2002
#Return-Path: <arnold@skeeve.com>
#Received: (from arnold@localhost)
#	by skeeve.com (8.11.6/8.11.6) id g49ER3K27925
#	for arnold; Thu, 9 May 2002 17:27:03 +0300
#Date: Thu, 9 May 2002 17:27:03 +0300
#From: Aharon Robbins <arnold@skeeve.com>
#Message-Id: <200205091427.g49ER3K27925@skeeve.com>
#To: arnold@skeeve.com
#Subject: fixme
#X-SpamBouncer: 1.4 (10/07/01)
#X-SBRule: Pattern Match (Other Patterns) (Score: 4850)
#X-SBRule: Pattern Match (Spam Phone #) (Score: 0)
#X-SBClass: Blocked
#Status: O
#
#Path: ord-read.news.verio.net!dfw-artgen!iad-peer.news.verio.net!news.verio.net!fu-berlin.de!uni-berlin.de!host213-120-137-48.in-addr.btopenworld.COM!not-for-mail
#From: laura@madonnaweb.com (laura fairhead)
#Newsgroups: comp.lang.awk
#Subject: bug in gawk3.1.0 regex code
#Date: Wed, 08 May 2002 23:31:40 GMT
#Organization: that'll be the daewooo :)
#Lines: 211
#Message-ID: <3cd9b0f7.29675926@NEWS.CIS.DFN.DE>
#Reply-To: laura@madonnaweb.com
#NNTP-Posting-Host: host213-120-137-48.in-addr.btopenworld.com (213.120.137.48)
#X-Trace: fu-berlin.de 1020900891 18168286 213.120.137.48 (16 [53286])
#X-Newsreader: Forte Free Agent 1.21/32.243
#Xref: dfw-artgen comp.lang.awk:13059
#
#
#I believe I've just found a bug in gawk3.1.0 implementation of
#extended regular expressions. It seems to be down to the alternation
#operator; when using an end anchor '$' as a subexpression in an
#alternation and the entire matched RE is a nul-string it fails
#to match the end of string, for example;
#
#gsub(/$|2/,"x")
#print
#
#input           = 12345
#expected output = 1x345x
#actual output   = 1x345
#
#The start anchor '^' always works as expected;
#
#gsub(/^|2/,"x")
#print
#
#input           = 12345
#expected output = x1x345
#actual output   = x1x345
#
#This was with POSIX compliance enabled althought that doesn't
#effect the result.
#
#I checked on gawk3.0.6 and got exactly the same results however
#gawk2.15.6 gives the expected results.
#
#I'm about to post a bug report about this into gnu.utils.bug
#but I thought I'd post it here first in case anyone has
#any input/comments/whatever ....
#
#Complete test results were as follows;
#
#input          12345
#output         gsub(/regex/,"x",input)
#
#regex          output
#(^)            x12345
#($)            12345x
#(^)|($)        x12345x
#($)|(^)        x12345x
#(2)            1x345
#(^)|2          x1x345
#2|(^)          x1x345
#($)|2          1x345
#2|($)          1x345
#(2)|(^)        x1x345
#(^)|(2)        x1x345
#(2)|($)        1x345
#($)|(2)        1x345
#.((2)|(^))     x345
#.((^)|(2))     x345
#.((2)|($))     x34x
#.(($)|(2))     x34x
#x{0}((2)|(^))  x1x345
#x{0}((^)|(2))  x1x345
#x{0}((2)|($))  1x345
#x{0}(($)|(2))  1x345
#x*((2)|(^))    x1x345
#x*((^)|(2))    x1x345
#x*((2)|($))    1x345
#x*(($)|(2))    1x345
#
#Here's the test program I used, a few of the cases use ERE {n[,[m]]}
#operators so that will have to be commented out or have a check
#added or something (should have put a conditional in I know... ;-)
#
#~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
#
BEGIN{

TESTSTR="12345"

print "input          "TESTSTR
print "output         gsub(/regex/,\"x\",input)"
print ""

print "regex          output"
$0=TESTSTR
gsub(/(^)/,"x")
print "(^)            "$0

$0=TESTSTR
gsub(/($)/,"x")
print "($)            "$0

$0=TESTSTR
gsub(/(^)|($)/,"x")
print "(^)|($)        "$0

$0=TESTSTR
gsub(/($)|(^)/,"x")
print "($)|(^)        "$0

$0=TESTSTR
gsub(/2/,"x")
print "(2)            "$0

$0=TESTSTR
gsub(/(^)|2/,"x")
print "(^)|2          "$0

$0=TESTSTR
gsub(/2|(^)/,"x")
print "2|(^)          "$0

$0=TESTSTR
gsub(/($)|2/,"x")
print "($)|2          "$0

$0=TESTSTR
gsub(/2|($)/,"x")
print "2|($)          "$0

$0=TESTSTR
gsub(/(2)|(^)/,"x")
print "(2)|(^)        "$0

$0=TESTSTR
gsub(/(^)|(2)/,"x")
print "(^)|(2)        "$0

$0=TESTSTR
gsub(/(2)|($)/,"x")
print "(2)|($)        "$0

$0=TESTSTR
gsub(/($)|(2)/,"x")
print "($)|(2)        "$0

$0=TESTSTR
gsub(/.((2)|(^))/,"x")
print ".((2)|(^))     "$0

$0=TESTSTR
gsub(/.((^)|(2))/,"x")
print ".((^)|(2))     "$0

$0=TESTSTR
gsub(/.((2)|($))/,"x")
print ".((2)|($))     "$0

$0=TESTSTR
gsub(/.(($)|(2))/,"x")
print ".(($)|(2))     "$0

# $0=TESTSTR
# gsub(/x{0}((2)|(^))/,"x")
# print "x{0}((2)|(^))  "$0
# 
# $0=TESTSTR
# gsub(/x{0}((^)|(2))/,"x")
# print "x{0}((^)|(2))  "$0
# 
# $0=TESTSTR
# gsub(/x{0}((2)|($))/,"x")
# print "x{0}((2)|($))  "$0
# 
# $0=TESTSTR
# gsub(/x{0}(($)|(2))/,"x")
# print "x{0}(($)|(2))  "$0

$0=TESTSTR
gsub(/x*((2)|(^))/,"x")
print "x*((2)|(^))    "$0

$0=TESTSTR
gsub(/x*((^)|(2))/,"x")
print "x*((^)|(2))    "$0

$0=TESTSTR
gsub(/x*((2)|($))/,"x")
print "x*((2)|($))    "$0

$0=TESTSTR
gsub(/x*(($)|(2))/,"x")
print "x*(($)|(2))    "$0

# $0=TESTSTR
# gsub(/x{0}^/,"x")
# print "x{0}^          "$0
# 
# $0=TESTSTR
# gsub(/x{0}$/,"x")
# print "x{0}$          "$0
# 
# $0=TESTSTR
# gsub(/(x{0}^)|2/,"x")
# print "(x{0}^)|2      "$0
# 
# $0=TESTSTR
# gsub(/(x{0}$)|2/,"x")
# print "(x{0}$)|2      "$0


}
#
#~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
#
#byefrom
#
#-- 
#laura fairhead  # laura@madonnaweb.com  http://lf.8k.com
#                # if you are bored crack my sig.
#1F8B0808CABB793C0000666667002D8E410E83300C04EF91F2877D00CA138A7A
#EAA98F30C494480157B623C4EF1B508FDED1CEFA9152A23DE35D661593C5318E
#630C313CD701BE92E390563326EE17A3CA818F5266E4C2461547F1F5267659CA
#8EE2092F76C329ED02CA430C5373CC62FF94BAC6210B36D9F9BC4AB53378D978
#80F2978A1A6E5D6F5133B67B6113178DC1059526698AFE5C17A5187E7D930492
