
function f1(b) { b = b "c"; print f(b); }

function f(a) { a = a "b"; return a; }

BEGIN { A = "a"; f1(A); }
