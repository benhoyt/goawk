BEGIN {
    cat  = "cat ; exit 3"
    print system("echo xxx | (cat ; exit 4)")

    print "YYY" | cat

    print close(cat)

    echo = "echo boo ; exit 5"
    echo | getline boo
    print "got", boo

    print close(echo)
}
