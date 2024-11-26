
# TODO: add tests for these

# Set fields from array a according to the order in OFIELDS, which must have
# field numbers as keys (from 1 to N) and field names as values, for example
# OFIELDS[1] = "name"; OFIELDS[2] = "age".
function setfields(a,    i) {
    for (i=1; i in OFIELDS; i++) {
        $i = a[OFIELDS[i]]
    }
    NF = i-1
}

# Call setfields(a) and then print the current row.
function printfields(a) {
    setfields(a)
    print
}

# Print the header (field names) from OFIELDS
function printheader(    i) {
    for (i=1; i in OFIELDS; i++) {
        $i = OFIELDS[i]
    }
    NF = i-1
    print
}

# Delete the nth field from $0. If num is given, delete num fields starting
# from the nth field.
function delfield(n, num,    i) {
    if (n < 1 || n > NF || num < 0) {
        $1 = $1  # ensure $0 gets rewritten
        return
    }
    if (num == 0) {
        num = 1
    }
    if (num > NF-n+1) {
        num = NF-n+1
    }
    for (i=n; i<=NF-num; i++) {
        $i = $(i+num)
    }
    NF -= num
}

# Insert a new empty field just before the nth field in $0. If num is given,
# insert num empty fields just before the nth field.
function insfield(n, num,    i) {
    if (n < 1 || num < 0) {
        $1 = $1  # ensure $0 gets rewritten
        return
    }
    if (num == 0) {
        num = 1
    }
    for (i=NF; i>=n; i--) {
        $(i+num) = $i
    }
    for (i=n; i<n+num; i++) {
        $i = ""
    }
}

# Return the number of the given named field, or 0 if there's no field with
# that name. Only works in -H/header mode.
function fieldnum(name,    i) {
    for (i=1; i in FIELDS; i++) {
        if (FIELDS[i] == name) {
            return i
        }
    }
    return 0
}
