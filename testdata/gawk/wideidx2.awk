# Date: Thu, 27 Apr 2006 20:59:03 +0100
# From: Lee Haywood <ljhaywood2@googlemail.com>
# Subject: gawk multi-byte support bugs, assertion bug and fix.
# To: bug-gawk@gnu.org
# Message-id: <60962be00604271259na0d8fdayb9d0c69a853216e8@mail.gmail.com>
# MIME-version: 1.0
# Content-type: multipart/alternative;
#  boundary="----=_Part_10136_920879.1146167943492"
# Status: RO
# 
# ------=_Part_10136_920879.1146167943492
# Content-Type: text/plain; charset=ISO-8859-1
# Content-Transfer-Encoding: quoted-printable
# Content-Disposition: inline
# 
# 
# Firstly, I have been getting the following error from version 3.1.5.
# 
#     awk: node.c:515: unref: Assertion `(tmp->flags & 4096) !=3D 0' failed.
# 
# In mk_number() in node.c the MBS_SUPPORT code is inside the GAWKDEBUG
# section - moving it outside explicitly clears the string values, which
# prevents the assertion error from occurring.  The corrected version is
# shown at the end of this message.
# 
# As an aside, I also noticed that n->wstptr is not cleared by
# set_field() and set_record() in field.c when the flags are set to
# exclude WSTRCUR.  However, I do not have a test case to show if
# changing them makes any difference.
# 
# A second problem also occurs when gawk 3.1.5 is compiled with
# multi-byte character support (MBS_SUPPORT).  The following code should
# change the index of the substring "bc" from 2 to 3, but it gets
# reported as 2 in both cases - which is obviously disastrous.
# 
#     awk 'BEGIN {
#             Value =3D "abc"
# 
#             print "Before <" Value "> ",
#                   index( Value, "bc" )
# 
#             sub( /bc/, "bbc", Value )
# 
#             print "After  <" Value ">",
#                   index( Value, "bc" )
#         }'
# 
# Compiling with MBS_SUPPORT undefined makes these problems go away.
# 
# /* mk_number --- allocate a node with defined number */
# 
# NODE *
# mk_number(AWKNUM x, unsigned int flags)
# {
#         register NODE *r;
# 
#         getnode(r);
#         r->type =3D Node_val;
#         r->numbr =3D x;
#         r->flags =3D flags;
# #if defined MBS_SUPPORT
#         r->wstptr =3D NULL;
#         r->wstlen =3D 0;
# #endif /* MBS_SUPPORT */
# #ifdef GAWKDEBUG
#         r->stref =3D 1;
#         r->stptr =3D NULL;
#         r->stlen =3D 0;
# #if defined MBS_SUPPORT
#         r->flags &=3D ~WSTRCUR;
# #endif /* MBS_SUPPORT */
# #endif /* GAWKDEBUG */
#         return r;
# }
# 
# Thanks.
# 
# --
# Lee Haywood.

BEGIN {
	Value = "abc"

	print "Before <" Value "> ", index( Value, "bc" )
 
	sub( /bc/, "bbc", Value )

	print "After  <" Value ">", index( Value, "bc" )
}
