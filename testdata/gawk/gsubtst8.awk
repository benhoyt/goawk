{
        OFS = " " $2 " "
        gsub("foo", "_", OFS)
        print $1, $2
}
