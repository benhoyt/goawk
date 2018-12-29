#!/bin/awk -f

function crash () {
    exit 1
}

function true (a,b,c) {
    return 1
}

BEGIN {
    if (ARGV[2] == 1) {
        print "<BEGIN CONTEXT> true(1, crash()) => crash properly."
        true(1, crash())
	# ADR: Added:
	delete ARGV[2]
    }
}

{
        print "<RULE CONTEXT> true(1, crash()) => do not crash properly."
        true(1, crash())
}

# FdF
