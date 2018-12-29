#Date: Mon, 7 Jun 2004 10:40:28 -0500
#From: mary1john8@earthlink.net
#To: arnold@skeeve.com
#Subject: gawk internal errors
#Message-ID: <20040607154028.GA2457@apollo>
#
#Hello,
#
#    gawk-3.1.3i internal errors:
#
#[1]
#
#$> ./gawk 'BEGIN { for (i in a) delete a; }'
BEGIN { for (i in a) delete a; }
#gawk: fatal error: internal error
#Aborted
#
#------------------------------------------------------------------
#--- awkgram.y.orig	2004-06-07 09:42:14.000000000 -0500
#+++ awkgram.y	2004-06-07 09:45:58.000000000 -0500
#@@ -387,7 +387,7 @@
# 		 * Check that the body is a `delete a[i]' statement,
# 		 * and that both the loop var and array names match.
# 		 */
#-		if ($8 != NULL && $8->type == Node_K_delete) {
#+		if ($8 != NULL && $8->type == Node_K_delete && $8->rnode != NULL) {
# 			NODE *arr, *sub;
# 
# 			assert($8->rnode->type == Node_expression_list);
#------------------------------------------------------------------
#
#
#[2]
#
#$> ./gawk 'BEGIN { printf("%3$*10$.*1$s\n", 20, 10, "hello"); }'
#gawk: fatal error: internal error
#Aborted
#
#------------------------------------------------------------------
#--- builtin.c.orig	2004-06-07 10:04:20.000000000 -0500
#+++ builtin.c	2004-06-07 10:06:08.000000000 -0500
#@@ -780,7 +780,10 @@
# 					s1++;
# 					n0--;
# 				}
#-
#+				if (val >= num_args) {
#+					toofew = TRUE;
#+					break;
#+				}
# 				arg = the_args[val];
# 			} else {
# 				parse_next_arg();
#------------------------------------------------------------------
#
#
#    Finally, a test for the rewritten get_src_buf():
#
#$> AWKBUFSIZE=2 make check
#
#I get 3 failed tests. Not sure this is of any interest.
#
#
#Thanks,
#John
