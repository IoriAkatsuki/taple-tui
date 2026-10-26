//go:build windows && cgo

#include "loader.h"
#define WIN32_LEAN_AND_MEAN
#include <windows.h>

void *tf_loader_open(void) {
  // Never search the working directory or PATH for executable DLL code.
  return LoadLibraryExW(L"vulkan-1.dll", NULL, LOAD_LIBRARY_SEARCH_SYSTEM32);
}

PFN_vkGetInstanceProcAddr tf_loader_proc(void *loader) {
  return (PFN_vkGetInstanceProcAddr)GetProcAddress((HMODULE)loader,
                                                   "vkGetInstanceProcAddr");
}

void tf_loader_close(void *loader) { FreeLibrary((HMODULE)loader); }
