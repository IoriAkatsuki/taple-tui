#ifndef TAPEFOX_TEST_WINDOWS_H
#define TAPEFOX_TEST_WINDOWS_H
#include <stdint.h>
#include <wchar.h>
typedef void *HMODULE;
typedef void (*FARPROC)(void);
#define LOAD_LIBRARY_SEARCH_SYSTEM32 0x00000800
HMODULE LoadLibraryExW(const wchar_t *path, void *file, uint32_t flags);
FARPROC GetProcAddress(HMODULE module, const char *name);
int FreeLibrary(HMODULE module);
#endif
