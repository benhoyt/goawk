# The first of these used to core dump gawk. As of 7/2013 it's fixed.
# This file is to make sure that remains true.
{match($0, /(([^ \?.]*\?pos=ad |([^ \?.]*\?pos=(jj|va) )地\?pos=dev ){0,2})/ , arr)}  { if(arr[0]) print arr[1], arr[4], $6}
{match($0, /(([^ \?.]*\?pos=ad |([^ \?.]*\?pos=(jj|va) )[地]\?pos=dev ){0,2})/ , arr)}  { if(arr[0]) print arr[1], arr[4], $6} 
