BEGIN {
    a[i] = "null"                         # i is initially undefined
    for (i in a) {                        # i is null string
        print length(i), a[i] # , typeof(i)  # 0 null
        print (i==0), (i=="")             # 1 1  should be  0 1
    }
    print a[""]                           # null
    print a[0]                            #

    b[$2] = "null also"                   # $2 is also undefined
    for (j in b) {
        print length(j), a[j] # , typeof(i)  # 0 null
        print (j==0), (j=="")             # 1 1  should be  0 1
    }
    print b[""]                           # null
    print b[0]                            #
}
