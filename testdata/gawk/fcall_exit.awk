#!/bin/awk -f

function crash () {
    exit 1
}

function true (a,b,c) {
    return 0
}

BEGIN {
    if (ARGV[1] == 1) {
        print "true(1, 1, crash()) => crash properly."
        true(1, 1, crash())
    } else if (ARGV[1] == 2) {
        print "true(1, crash(), 1) => do not crash properly."
        true(1, crash(),1)
    } else {
        print "true(1, crash()) => do not crash properly."
        true(1, crash())
    }
}

# FdF
