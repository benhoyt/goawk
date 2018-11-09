# Generate and display the Mandelbrot set

BEGIN {
    # Constants to determine size and coordinates of grid
    width = 150; height = 50
    min_x = -2.1; max_x = 0.6
    min_y = -1.2; max_y = 1.2
    iters = 32

    # "Colors", from '.' (diverges fastest) to '@' (diverges slowly),
    # and then ' ' for doesn't diverge within `iters` iterations.
    colors[0] = "."
    colors[1] = "-"
    colors[2] = "+"
    colors[3] = "*"
    colors[4] = "%%"
    colors[5] = "#"
    colors[6] = "$"
    colors[7] = "@"
    colors[8] = " "

    # Loop from top to bottom, and for each line left to right
    inc_y = (max_y-min_y) / height
    inc_x = (max_x-min_x) / width
    y = min_y
    for (row=0; row<height; row++) {
        x = min_x
        for (col=0; col<width; col++) {
            zr = zi = 0
            for (i=0; i<iters; i++) {
                # Main calculation: z = z^2 + c
                old_zr = zr
                zr = zr*zr - zi*zi + x
                zi = 2*old_zr*zi + y
                # Stop when magnitude is greater than 2
                if (zr*zr + zi*zi > 4) break
            }
            # Scale "color" according to how fast it diverged
            printf colors[int(i*8/iters)]
            x += inc_x
        }
        y += inc_y
        printf "\n"
    }
}
