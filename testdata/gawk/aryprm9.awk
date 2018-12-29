#!/usr/bin/gawk -f
BEGIN {

     for (i = 0; i < 100; i++)
         func_exec()
}

function func_exec(opaque)
{
     func_a(1, opaque)    #set additional argument, not expected by fname
}

function func_a(a,    b, loc1, loc2)
{
     b = 0            #unref Nnull_string
}
