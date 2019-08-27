// This test is intended to check that the whole stdlib is present and with the right parameter names

// Functions without optional arguments need only one line
// Functions with optional arguments need two lines - one with none of the optional arguments
// and the other with all of them

// TODO(sbarzowski) finish this
[
    // extVar and native are skipped here, because of the special setup required
    std.thisFile,
    std.type(x={}),
    std.length(x=[]),
    std.objectHas(o={}, f="fieldname"),
    std.objectFields(o={}),
    std.objectHasAll(o={}, f="fieldname"),
    std.prune(o={}),
//     // Types and reflection
//     "thisFile":        stringType,
//     "type":            g.newSimpleFuncType(stringType, "x"),
//     "length":          g.newSimpleFuncType(numberType, "x"),
//     "objectHas":       g.newSimpleFuncType(boolType, "o", "f"),
//     "objectFields":    g.newSimpleFuncType(arrayOfString, "o"),
//     "objectHasAll":    g.newSimpleFuncType(boolType, "o", "f"),
//     "objectFieldsAll": g.newSimpleFuncType(arrayOfString, "o"),
//     "prune":           g.newSimpleFuncType(anyObjectType, "o"),
//     "mapWithKey":      g.newSimpleFuncType(anyObjectType, "func", "obj"),

//     // Mathematical utilities
//     "abs":      g.newSimpleFuncType(numberType, "n"),
//     "sign":     g.newSimpleFuncType(numberType, "n"),
//     "max":      g.newSimpleFuncType(numberType, "a", "b"),
//     "min":      g.newSimpleFuncType(numberType, "a", "b"),
//     "pow":      g.newSimpleFuncType(numberType, "x", "n"),
//     "exp":      g.newSimpleFuncType(numberType, "x"),
//     "log":      g.newSimpleFuncType(numberType, "x"),
//     "exponent": g.newSimpleFuncType(numberType, "x"),
//     "mantissa": g.newSimpleFuncType(numberType, "x"),
//     "floor":    g.newSimpleFuncType(numberType, "x"),
//     "ceil":     g.newSimpleFuncType(numberType, "x"),
//     "sqrt":     g.newSimpleFuncType(numberType, "x"),
//     "sin":      g.newSimpleFuncType(numberType, "x"),
//     "cos":      g.newSimpleFuncType(numberType, "x"),
//     "tan":      g.newSimpleFuncType(numberType, "x"),
//     "asin":     g.newSimpleFuncType(numberType, "x"),
//     "acos":     g.newSimpleFuncType(numberType, "x"),
//     "atan":     g.newSimpleFuncType(numberType, "x"),

//     // Assertions and debugging
//     "assertEqual": g.newSimpleFuncType(boolType, "a", "b"),

//     // String Manipulation

//     "toString":            g.newSimpleFuncType(stringType, "a"),
//     "codepoint":           g.newSimpleFuncType(numberType, "str"),
//     "char":                g.newSimpleFuncType(stringType, "n"),
//     "substr":              g.newSimpleFuncType(stringType, "s", "from", "len"),
//     "findSubstr":          g.newSimpleFuncType(arrayOfNumber, "pat", "str"),
//     "startsWith":          g.newSimpleFuncType(boolType, "a", "b"),
//     "endsWith":            g.newSimpleFuncType(boolType, "a", "b"),
//     "split":               g.newSimpleFuncType(arrayOfString, "str", "c"),
//     "splitLimit":          g.newSimpleFuncType(arrayOfString, "str", "c", "maxsplits"),
//     "strReplace":          g.newSimpleFuncType(stringType, "str", "from", "to"),
//     "asciiUpper":          g.newSimpleFuncType(stringType, "str"),
//     "asciiLower":          g.newSimpleFuncType(stringType, "str"),
//     "stringChars":         g.newSimpleFuncType(stringType, "str"),
//     "format":              g.newSimpleFuncType(stringType, "str", "vals"),
//     "escapeStringBash":    g.newSimpleFuncType(stringType, "str"),
//     "escapeStringDollars": g.newSimpleFuncType(stringType, "str"),
//     "escapeStringJson":    g.newSimpleFuncType(stringType, "str"),
//     "escapeStringPython":  g.newSimpleFuncType(stringType, "str"),

//     // Parsing

//     "parseInt":   g.newSimpleFuncType(numberType, "str"),
//     "parseOctal": g.newSimpleFuncType(numberType, "str"),
//     "parseHex":   g.newSimpleFuncType(numberType, "str"),
//     "parseJson":  g.newSimpleFuncType(jsonType, "str"),
//     "encodeUTF8": g.newSimpleFuncType(arrayOfNumber, "str"),
//     "decodeUTF8": g.newSimpleFuncType(stringType, "arr"),

//     // Manifestation

//     "manifestIni":        g.newSimpleFuncType(stringType, "v"),
//     "manifestPython":     g.newSimpleFuncType(stringType, "v"),
//     "manifestPythonVars": g.newSimpleFuncType(stringType, "v"),
//     "manifestJsonEx":     g.newSimpleFuncType(stringType, "value", "indent"),
//     "manifestYamlDoc":    g.newSimpleFuncType(stringType, "value"),
//     "manifestYamlStream": g.newSimpleFuncType(stringType, "value"),
//     "manifestXmlJsonml":  g.newSimpleFuncType(stringType, "value"),

//     // Arrays

//     "makeArray":     g.newSimpleFuncType(anyArrayType, "sz", "func"),
//     "count":         g.newSimpleFuncType(numberType, "arr", "x"),
//     "find":          g.newSimpleFuncType(arrayOfNumber, "value", "arr"),
//     "map":           g.newSimpleFuncType(anyArrayType, "func", "arr"),
//     "mapWithIndex":  g.newSimpleFuncType(anyArrayType, "func", "arr"),
//     "filterMap":     g.newSimpleFuncType(anyArrayType, "filter_func", "map_func", "arr"),
//     "filter":        g.newSimpleFuncType(anyArrayType, "func", "arr"),
//     "foldl":         g.newSimpleFuncType(anyType, "func", "arr", "init"),
//     "foldr":         g.newSimpleFuncType(anyType, "func", "arr", "init"),
//     "range":         g.newSimpleFuncType(arrayOfNumber, "from", "to"),
//     "join":          g.newSimpleFuncType(stringOrArray, "sep", "arr"),
//     "lines":         g.newSimpleFuncType(arrayOfString, "arr"),
//     "flattenArrays": g.newSimpleFuncType(anyArrayType, "arrs"),
//     // TODO(sbarzowski) support optional args
//     // "sort": g.newSimpleFuncType(anyArrayType, "arr", keyF=id),
//     // Don't we have keyF for uniq? Perhaps we should?
//     "uniq": g.newSimpleFuncType(anyArrayType, "arr"),

//     // Sets

//     // TODO(sbarzowski) support optional args
//     // "set": g.newSimpleFuncType(comparableArray, "arr", keyF=id)
//     // "setInter": g.newSimpleFuncType(comparableArray, "a", b, keyF=id)
//     // "setUnion": g.newSimpleFuncType(comparableArray, "a", b, keyF=id)
//     // "setDiff": g.newSimpleFuncType(comparableArray, "a", b, keyF=id)
//     // "setMember": g.newSimpleFuncType(comparableArray, "x", arr, keyF=id)

//     // Encoding

//     "base64":            g.newSimpleFuncType(stringType, "v"),
//     "base64DecodeBytes": g.newSimpleFuncType(numberType, "s"),
//     "base64Decode":      g.newSimpleFuncType(stringType, "s"),
//     "md5":               g.newSimpleFuncType(stringType, "s"),

//     // JSON Merge Patch

//     "mergePatch": g.newSimpleFuncType(anyType, "target", "patch"),

//     // Debugging

//     "trace": g.newSimpleFuncType(anyType, "str", "rest"),

//     // Undocumented
//     "manifestJson":     g.newSimpleFuncType(stringType, "value"),
//     "objectHasEx":      g.newSimpleFuncType(boolType, "obj", "fname", "hidden"),
//     "objectFieldsEx":   g.newSimpleFuncType(arrayOfString, "obj", "hidden"),
//     "flatMap":          g.newSimpleFuncType(anyArrayType, "func", "arr"),
//     "modulo":           g.newSimpleFuncType(numberType, "x", "y"),
//     "slice":            g.newSimpleFuncType(arrayOfString, "indexable", "index", "end", "step"),
//     "primitiveEquals":  g.newSimpleFuncType(boolType, "x", "y"),
//     "mod":              g.newSimpleFuncType(stringOrNumber, "a", "b"),
//     "native":           g.newSimpleFuncType(anyFunctionType, "x"),
]
