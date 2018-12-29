BEGIN {
    n = split("-1.2e+931 1.2e+931", a)
    for (i=1; i<=n; ++i)
        print a[i], +a[i]
}
