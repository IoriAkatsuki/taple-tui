#include "../loader_windows.c"
#include <assert.h>
#include <string.h>

static int available, load_calls, close_calls;
static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL proc(VkInstance instance,
                                                     const char *name) {
  (void)instance;
  (void)name;
  return NULL;
}

HMODULE LoadLibraryExW(const wchar_t *path, void *file, uint32_t flags) {
  assert(wcscmp(path, L"vulkan-1.dll") == 0);
  assert(file == NULL && flags == 0x00000800);
  load_calls++;
  return available ? (HMODULE)1 : NULL;
}

FARPROC GetProcAddress(HMODULE module, const char *name) {
  assert(module == (HMODULE)1 && strcmp(name, "vkGetInstanceProcAddr") == 0);
  return (FARPROC)proc;
}

int FreeLibrary(HMODULE module) {
  assert(module == (HMODULE)1);
  close_calls++;
  return 1;
}

int main(void) {
  assert(tf_loader_open() == NULL && load_calls == 1);
  available = 1;
  void *loader = tf_loader_open();
  assert(loader != NULL && load_calls == 2);
  assert(tf_loader_proc(loader) == proc);
  tf_loader_close(loader);
  assert(close_calls == 1);
  return 0;
}
