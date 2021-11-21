{
    for (i = 1; i <= NF; i++)
        counts[tolower($i)]++
}

END {
    for (k in counts)
        print k, counts[k]
}
