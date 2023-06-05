#!/usr/bin/awk -f
BEGIN { init() }

function init(   i,isStructure,isUngron) {
  for (i = 1; i < ARGC; i++) {
    if ((isUngron = "-u"==ARGV[i]) || (isStructure = "-s"==ARGV[i])) {
      delete ARGV[i]
      break
    }
  }
  Pos=1
  RS="\x01" # this allows to read whole input at once
  if ((getline In) < 0) die("I/O error")

  isUngron ? ungron() : gron(isStructure)
}
function gron(isStructure) {
  # --- parse JSON ---
  split("", Asm); AsmLen=0

  if (!ELEMENT() || Pos <= length(In))
    dieAtPos("Can't parse JSON")
    #      dbgA("--- JSON asm:",Asm)

    # --- generate GRON ---
  split("",AlreadyTracked)
  split("",Stack); split("",PathStack)
  Depth = 0
  generateGron(isStructure)
}
function ungron(   i,instr) {
  split("", Asm); AsmLen=0 # Gron asm
  split("", JsonAsm); JsonAsmLen=0

  if (!STATEMENTS() || Pos <= length(In))
    dieAtPos("Can't parse GRON")

    # --- ungron (gron asm -> json asm) ---
    #  dbgA("--- Gron asm:",Asm)

  split("", AddrType)  # addr -> type
  split("", AddrValue) # addr -> value
  split("", AddrCount) # addr -> segment count
  split("", AddrKey)   # addr -> last segment name

  for (i=0; i<AsmLen; i++) {
    instr = Asm[i]

    if("record" == instr) {
      split("",Path)
      split("",Types)
      split("",Value) # [ type, value ]
    }
    else if (isSegmentType(instr)) { arrPush(Types, instr); arrPush(Path, Asm[++i]) }
    else if ("value" == instr) {
      instr = Asm[++i]
      split("",Value)
      Value[0] = instr
      if (isValueHolder(instr))
        Value[1] = Asm[++i]
    } else if ("end" == instr) { processRecord() }
  }
  generateJsonAsm()
  #  dbgA("--- JSON asm:",JsonAsm)

  # --- generate JSON ---
  #    Indent = ENVIRON["Indent"] + 0
  Indent = 2
  for (i=0; i<Indent; i++)
    IndentStr=IndentStr " "
  Open["object"]="{" ; Close["object"]="}" ; Opens["end_object"] = "object"
  Open["array"] ="[" ; Close["array"] ="]" ; Opens["end_array"]  = "array"
  split("",Stack)
  Depth = 0
  generateJson()
}
#function dbgA(title,arr,   i) { print title; for(i=0;i in arr;i++) printf "%2s : %s\n", i,arr[i] }

# --- JSON ---
function tryParseDigits(res) { return tryParse("0123456789", res) }
function NUMBER(    res) {
  return (tryParse1("-", res) || 1) &&
    (tryParse1("0", res) || tryParse1("123456789", res) && (tryParseDigits(res)||1)) &&
    (tryParse1(".", res) ? tryParseDigits(res) : 1) &&
    (tryParse1("eE", res) ? (tryParse1("-+",res)||1) && tryParseDigits(res) : 1) &&
    asm("number") && asm(res[0])
}
function tryParseHex(res) { return tryParse1("0123456789ABCDEFabcdef", res) || err(res) }
function err(res) { res[1]=1; return 0 }
function tryParseCharacters(res) { while (tryParseCharacter(res)); return !res[1] }
function tryParseCharacter(res) { return tryParseSafeChar(res) || tryParseEscapeChar(res) }
function tryParseEscapeChar(res) {
  return tryParse1("\\", res) ? tryParse1("\"\\/bfnrt", res) || tryParseU(res) : 0
}
function tryParseU(res) { return tryParse1("u", res) ? tryParseHex(res) && tryParseHex(res) && tryParseHex(res) && tryParseHex(res) : 0 }
function tryParseSafeChar(res,   c) {
  c = nextChar()
  # https://github.com/antlr/grammars-v4/blob/master/json/JSON.g4#L56
  if (c != "\"" && c != "\\" && c > "\x1F") {
    Pos++
    res[0] = res[0] c
    return 1
  }
  return 0
}
function STRING(isKey,    res) {
  return tryParse1("\"",res) && asm(isKey ? "key" : "string") &&
    tryParseCharacters(res) &&
    tryParse1("\"",res) &&
    asm(res[0])
}
function WS() { return tryParse("\t\n\r ") || 1 }
function WS1() { return tryParse("\t ") || 1 }
function VALUE() {
  return OBJECT() ||
    ARRAY()  ||
    STRING() ||
    NUMBER() ||
    tryParseExact("true") && asm("true") ||
    tryParseExact("false") && asm("false") ||
    tryParseExact("null") && asm("null")
}
function OBJECT() {
  return tryParse1("{") && asm("object") &&
    (WS() && tryParse1("}") ||
    MEMBERS() && tryParse1("}")) &&
    asm("end_object")
}
function ARRAY() {
  return tryParse1("[") && asm("array") &&
    (WS() && tryParse1("]") ||
    ELEMENTS() && tryParse1("]")) &&
    asm("end_array")
}
function MEMBERS() {
  return MEMBER() && (tryParse1(",") ? MEMBERS() : 1)
}
function ELEMENTS() {
  return ELEMENT() && (tryParse1(",") ? ELEMENTS() : 1)
}
function MEMBER() {
  return WS() && STRING(1) && WS() && tryParse1(":") && ELEMENT()
}
function ELEMENT() {
  return WS() && VALUE() && WS()
}
# --- GRON ---
function STATEMENTS() {
  for(tryParse("\n");nextChar();tryParse("\n")) {
    if (!STATEMENT())
      return 0
  }
  return 1
}
function STATEMENT() {
  return asm("record") &&
    PATH() && WS1() && tryParse1("=") && WS1() && asm("value") && VALUE_GRON() && (tryParse1(";")||1) &&
    asm("end")
}
function PATH() {
  return BARE_WORD() && SEGMENTS()
}
function BARE_WORD(    word) {
  return tryParse1("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ$_", word) &&
    (tryParse("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ$_0123456789", word) || 1) &&
    asm("key") && asm("\"" word[0] "\"")
}
function SEGMENTS() {
  return SEGMENT() && SEGMENTS() || 1
}
function SEGMENT() {
  return tryParse1(".") && BARE_WORD() ||
    tryParse1("[") && KEY() && tryParse1("]")
}
function KEY(    idx) {
  return tryParse("0123456789", idx) &&
    asm("index") && asm(idx[0]) ||
    STRING(1)
}
function VALUE_GRON() {
  return STRING() ||
    NUMBER() ||
    tryParseExact("true") && asm("true") ||
    tryParseExact("false") && asm("false") ||
    tryParseExact("null") && asm("null") ||
    tryParseExact("{}") && asm("object") ||
    tryParseExact("[]") && asm("array")

}
# --- ungron ---
function isComplex(s) { return "object"==s || "array"==s }
function isSegmentType(s) { return "key" ==s || "index" ==s }
function isValueHolder(s) { return "string"==s || "number"==s }
function processRecord(   l, addr, type, value, i) {
  l = arrLen(Path)
  addr=""
  for (i=0; i<l; i++) {
    # build addr
    addr = addr (i>0?",":"") Path[i]
    if (i<l-1) {
      type = Types[i+1] == "key" ? "object" : "array"
      value = ""
    } else {
      type = Value[0]
      value = Value[1]
    }
    if (addr in AddrType && type != AddrType[addr]) {
      die("Conflicting types for " addr ": " type " and " AddrType[addr])
    }
    AddrType[addr] = type
    AddrValue[addr] = value
    AddrCount[addr] = i+1
    AddrKey[addr] = Path[i]
  }
}
function generateJsonAsm(   i,j,l, a,aPrev,aj,type,addrs,ends) {
  split("",Stack)
  ends["object"] = "end_object"
  ends["array"]  = "end_array"

  for (a in AddrType)
    arrPush(addrs, a)
  quicksort(addrs, 0, (l=arrLen(addrs))-1)
  for (i=0; i<l; i++) {
    a = addrs[i]
    type = AddrType[a]
    if (i>0) {
      aPrev = addrs[i-1]
      for (j=0; j<AddrCount[aPrev] - AddrCount[a] + isComplex(AddrType[aPrev]); j++)
        asmJson(ends[arrPop(Stack)])
        # determine the type of current container (object/array) - for array should not issue "key"
      for (j=i; AddrCount[a]-AddrCount[aj=addrs[j]] != 1; j--) {} # descend to addr of prev segment
      if ("array" != AddrType[aj]) {
        asmJson("key")
        asmJson(AddrKey[a]) # last segment in addr
      }
    }
    asmJson(type)
    if (isComplex(type))
      arrPush(Stack, type)
    if (isValueHolder(type))
      asmJson(AddrValue[a])
    if (i==l-1) { # last
      for (j=0; j<AddrCount[a] - (isComplex(type)?0:1); j++)
        asmJson(ends[arrPop(Stack)])
    }
  }
}
function asmJson(inst) { JsonAsm[JsonAsmLen++]=inst; return 1 }
function arrPush(arr, e) { arr[arr[-7]++] = e }
function arrPop(arr,   e,l) { e = arr[l=--arr[-7]]; if (l<0) die("Can't pop"); delete arr[l]; return e }
function arrLen(arr) { return +arr[-7] }
function die(msg) { print msg; exit 1 }
function dieAtPos(msg) { die(msg " at pos " Pos ": " posStr()) }
function posStr(   s) { return esc(s = substr(In,Pos,10)) (10==length(s)?"...":"") }
function esc(s) { gsub(/\n/, "\\n",s); return s }
# --- generate JSON ---
function generateJson(   i,instr,wasPrev) {
  for (i=0; i<JsonAsmLen; i++) {
    if (isComplex(instr = JsonAsm[i])) {
      p1(wasPrev, Open[instr] nlIndent(isEnd(JsonAsm[i+1]), Depth+1))
      Stack[++Depth]=instr; wasPrev=0 }
    else if ("key"==instr) {
      p1(wasPrev, JsonAsm[++i] ":" (Indent==0?"":" ")); wasPrev=0 }
    else if ("number"==instr || "string"==instr) {
      p1(wasPrev, JsonAsm[++i]); wasPrev=1 }
    else if (isSingle(instr)) {
      p1(wasPrev, instr); wasPrev=1 }
    else if (isEnd(instr)) {
      if (Stack[Depth] != Opens[instr]) die("end mismatch")
      p(nlIndent(isComplex(JsonAsm[i-1]), Depth-1) Close[Stack[Depth--]])
      wasPrev=1 }
    else { die("Wrong opcode") }
  }
  print ""
}
function p1(wasPrev,s) { p((wasPrev ? "," nlIndent(0, Depth) : "") s) }
function p(s) { printf "%s", s }
function nlIndent(unless, d,   i, s) { if (unless || Indent==0) return ""; for (i=0; i<d; i++) s = s IndentStr; return "\n" s }
# lib
function tryParseExact(s,    l) {
  if(substr(In,Pos,l=length(s))==s) { Pos += l; return 1 }
  return 0
}
function tryParse1(chars, res) { return tryParse(chars,res,1) }
function tryParse(chars, res, atMost,    i,c,s) {
  s=""
  while (index(chars, c = nextChar()) > 0 &&
         (atMost==0 || i++ < atMost) &&
         c != "") {
    s = s c
    Pos++
  }
  res[0] = res[0] s
  return s != ""
}
function nextChar() { return substr(In,Pos,1) }
function asm(inst) { Asm[AsmLen++]=inst; return 1 }

# -----
function generateGron(isStructure,   i, instr) {
  for (i=0; i<AsmLen; i++) {
    if (isComplex(instr = Asm[i])) {
      printRow(isStructure,"object"==instr?"{}":"[]")
      Stack[++Depth]=instr
      if (inArr()) { PathStack[Depth]=0 } }
    else if (isSingle(instr)) { printRow(isStructure,instr); incArrIdx() }
    else if (isEnd(instr)) { Depth--; incArrIdx() }
    else if ("key" == instr) { PathStack[Depth]=Asm[++i] }
    else if ("number"==instr || "string"==instr) { printRow(isStructure,Asm[++i]); incArrIdx() }
    else { print "Error at instruction#" i ": " instr; exit 1 }
  }
}
function isSingle(s) { return "true"==s || "false"==s || "null"==s }
function inArr() { return "array"==Stack[Depth] }
function isEnd(s) { return "end_object"==s || "end_array"==s }
function incArrIdx() { if (inArr()) PathStack[Depth]++ }
function printRow(isStructure, v) { isStructure ? printStructure(v) : printGron(v) }
function printStructure(v,    row,i,isArr,byIdx,segment) {
  row=""
  for(i=1; i<=Depth; i++) {
    segment = PathStack[i]
    byIdx = (isArr="array"==Stack[i]) || segment !~ /^"[a-zA-Z$_][a-zA-Z0-9$_]*"$/
    row = row (i==0||isArr?"":".") (isArr ? "[]" : byIdx ? segment : _unqote(segment))
  }
  if (row in AlreadyTracked) return
  AlreadyTracked[row]
  if ("{}" == v || "[]" == v) return
  row = row " = " v
  print row
}
function printGron(v,    row,i,byIdx,segment) {
  row="json"
  for(i=1; i<=Depth; i++) {
    segment = PathStack[i]
    byIdx = "array"==Stack[i] || segment !~ /^"[a-zA-Z$_][a-zA-Z0-9$_]*"$/
    row= row (i==0||byIdx?"":".") (byIdx ? "[" segment "]" : _unqote(segment))
  }
  row=row "=" v
  print row
}
function _unqote(text,    l) {
  return (l=length(text)) == 2 ? "" : substr(text, 2, l-2)
}
function natOrder(s1,s2, i1,i2,   c1, c2, n1,n2) {
  if (_digit(c1 = substr(s1,i1,1)) && _digit(c2 = substr(s2,i2,1))) {
    n1 = +c1; while(_digit(c1 = substr(s1,++i1,1))) { n1 = n1 * 10 + c1 }
    n2 = +c2; while(_digit(c2 = substr(s2,++i2,1))) { n2 = n2 * 10 + c2 }
    return n1 == n2 ? natOrder(s1, s2, i1, i2) : _cmp(n1, n2)
  }

  # consume till equal substrings
  while ((c1 = substr(s1,i1,1)) == (c2 = substr(s2,i2,1)) && c1 != "" && !_digit(c1)) {
    i1++; i2++
  }

  return _digit(c1) && _digit(c2) ? natOrder(s1, s2, i1, i2) : _cmp(c1, c2)
}
function _cmp(v1, v2) { return v1 > v2 ? 1 : v1 < v2 ? -1 : 0 }
function _digit(c) { return c >= "0" && c <= "9" }
function quicksort(data, left, right,   i, last) {
  if (left >= right)
    return
  _swap(data, left, int((left + right) / 2))
  last = left
  for (i = left + 1; i <= right; i++)
    if (natOrder(data[i], data[left],1,1) < 0)
      _swap(data, ++last, i)
  _swap(data, left, last)
  quicksort(data, left, last - 1)
  quicksort(data, last + 1, right)
}
function _swap(data, i, j,   temp) {
  temp = data[i]
  data[i] = data[j]
  data[j] = temp
}