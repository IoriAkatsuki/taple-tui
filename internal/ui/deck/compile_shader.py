#!/usr/bin/env python3
"""Rebuild SPIR-V and Metal MSL using libshaderc + spirv-cross (build tools only)."""
import ctypes as c
import pathlib
import subprocess
import sys

lib = c.CDLL("libshaderc_shared.so.1")
def bind(name, result, args):
    fn = getattr(lib, name)
    fn.restype, fn.argtypes = result, args
    return fn

init = bind("shaderc_compiler_initialize", c.c_void_p, [])
release = bind("shaderc_compiler_release", None, [c.c_void_p])
compile_spv = bind("shaderc_compile_into_spv", c.c_void_p, [c.c_void_p, c.c_char_p, c.c_size_t, c.c_int, c.c_char_p, c.c_char_p, c.c_void_p])
status = bind("shaderc_result_get_compilation_status", c.c_int, [c.c_void_p])
message = bind("shaderc_result_get_error_message", c.c_char_p, [c.c_void_p])
length = bind("shaderc_result_get_length", c.c_size_t, [c.c_void_p])
data = bind("shaderc_result_get_bytes", c.c_void_p, [c.c_void_p])
free_result = bind("shaderc_result_release", None, [c.c_void_p])
options_init = bind("shaderc_compile_options_initialize", c.c_void_p, [])
options_release = bind("shaderc_compile_options_release", None, [c.c_void_p])
optimize = bind("shaderc_compile_options_set_optimization_level", None, [c.c_void_p, c.c_int])
path = pathlib.Path(__file__).with_name("walkman.comp")
source = path.read_bytes()
compiler, options = init(), options_init()
if not compiler or not options:
    sys.exit("shaderc initialization failed")
optimize(options, 2)
result = compile_spv(compiler, source, len(source), 2, str(path).encode(), b"main", options)
try:
    if not result or status(result):
        sys.exit(message(result).decode() if result else "shaderc compilation failed")
    path.with_suffix(".comp.spv").write_bytes(c.string_at(data(result), length(result)))
finally:
    if result:
        free_result(result)
    options_release(options)
    release(compiler)

subprocess.run([sys.executable, str(path.with_name("compile_metal.py"))], check=True)
