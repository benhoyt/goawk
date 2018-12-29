BEGIN {
        pos[0] = 0
        posout[0] = 0
        split("00000779770060", f)      # f[1] is a strnum
        print typeof(f[1])
        pos[f[1]] = 1
        print typeof(f[1])
}
