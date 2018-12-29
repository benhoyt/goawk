# From: "T. X. G." <leopardie333@yahoo.com>
# Subject: Bug in regular expression \B using DFA
# Date: Wed, 16 Jul 2008 05:23:09 -0700 (PDT)
# To: bug-gawk@gnu.org
# 
# ~ gawk --version
# GNU Awk 3.1.6
# Copyright (C) 1989, 1991-2007 Free Software Foundation.
# 
# ......
# 
# You should have received a copy of the GNU General Public License
# along with this program. If not, see http://www.gnu.org/licenses/.
# 
# ~ LC_ALL=C gawk 'BEGIN{x="abcd";gsub(/\B/,":",x);print x}'
# a:b:cd
# 
# ~ LC_ALL=en_US.UTF-8 gawk 'BEGIN{x="abcd";gsub(/\B/,":",x);print x}'
# a:b:c:d
# 
# ~ GAWK_NO_DFA=1 gawk 'BEGIN{x="abcd";gsub(/\B/,":",x);print x}'
# a:b:c:d

BEGIN { x = "abcd"; gsub(/\B/,":",x); print x }
