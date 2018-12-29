# Tue Feb  4 12:20:10 IST 2003

# Misc functions tests, in case we start mucking around in the grammar again.

# Empty body shouldn't hurt anything:
function f() {}
BEGIN { f() }

# Using a built-in function name should manage the symbol table
# correctly:
function split(x) { return x }

function x(a) { return a }
