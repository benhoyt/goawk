# tests for assigning to a function within that function

#1 - should be bad
function test1 (r) { gsub(r, "x", test1) }
BEGIN { test1("") }
