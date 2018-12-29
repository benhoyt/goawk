# From arnold@f7.net  Sun Sep  5 12:30:53 2004
# Date: Fri, 3 Sep 2004 00:54:32 -0400 (EDT)
# From: William J Poser <wjposer@ldc.upenn.edu>
# To: arnold@skeeve.com
# Subject: gawk bug
# Message-ID: <20040903004347.W80049@lorax.ldc.upenn.edu>
# 
# Here is a revised version of my previous message, modified to describe
# the accompanying files.
# 
# IhSplit.awk should replicate every record with exactly one entry in the
# IH field, delete records lacking an IH field, and produce as many copies
# of records with two or more entries in the IH field as there are entries.
# In the latter case, the original IH field should be relabelled OIH and
# a new IH field be added at the beginning of the record.
# 
# This has worked properly for many years, since at least 1997. It worked properly with gawk 3.0.5
# and possibly later versions. Unfortunately I didn't keep track of exactly what version it
# broke on, but it was whatever came with Mandrake Linux 9.0. It continued to fail with version
# 3.1.2. However, the problem was eliminated with version 3.1.3 and remains
# eliminated in version 3.1.4.
# 
# The problem was that an apparently random subset of records would loose some
# or all of their fields. Running the script on the same input always produces
# the same output with the same errors.
# 
# The file Input is a subset of a real lexicon that produces errors using
# gawk 3.1.2. GoodOutput is the expected output. BadOutput is the erroneous
# output. A diff will show that there are actually two errors. One record
# has fields stripped as described above. Another is omitted in its entirety.
# 
# 
# Bill Poser, Linguistics, University of Pennsylvania
# http://www.ling.upenn.edu/~wjposer/ billposer@alum.mit.edu
# ----------------------------------------------------------------------------
#For each record that contains multiple items in its inverse headword (IH)
#field, generate a set of new records each containing exactly one item
#in the inverse headword field, otherwise copies of the original.

function CleanUp() #Clean up for next input record.
{
  for(i in rec) delete rec[i];
}

BEGIN {
RS = "";
FS = "\n?%"
}
{

# First, create an associative array with the tags as indices.
  for(i = 2; i <= NF; i++) { # The leading FS creates an initial empty field
       split($i, f, ":");
       rec[f[1]]=substr($i,index($i,":")+1);
  }

  if(!("IH" in rec)) next;

# Parse out the inverse headwords

     items = split(rec["IH"],ihs,"/");

# Replace the old IH field.

     sub(/%IH:/,"%OIH:",$0);

# Generate a new copy of the record for each inverse headword

       for(i = 1; i <= items; i++){
	 entries+=1;
         printf("%%IH:%s\n",ihs[i]);
         printf("%s\n\n",$0);
       }
       CleanUp();
  }
