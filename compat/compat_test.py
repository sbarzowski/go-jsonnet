import ctypes
lib = ctypes.CDLL('./compat.so')
lib.test2()
#lib.run_jsonnet("2 + 2")
err = ctypes.c_int()
lib.jsonnet_evaluate_snippet2.argtypes = [
    ctypes.c_char_p,
    ctypes.c_char_p,
    ctypes.POINTER(ctypes.c_int),
]
lib.jsonnet_evaluate_snippet2.restype = ctypes.c_char_p
res = lib.jsonnet_evaluate_snippet2(b"my_file", b"2 + 2", ctypes.byref(err))
print(repr(res))
