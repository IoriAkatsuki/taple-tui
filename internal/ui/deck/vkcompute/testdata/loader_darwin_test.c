#include <assert.h>
#include <string.h>
#define dlopen fake_dlopen
#define dlsym fake_dlsym
#define dlclose fake_dlclose
#include "../loader_darwin.c"

static const char *available;
static int close_calls;
static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL proc(VkInstance instance,
                                                     const char *name) {
  (void)instance;
  (void)name;
  return NULL;
}

void *fake_dlopen(const char *path, int flags) {
  assert(path[0] == '/' || strncmp(path, "@rpath/", 7) == 0);
  assert(flags == (RTLD_NOW | RTLD_LOCAL));
  return available && strcmp(path, available) == 0 ? (void *)1 : NULL;
}

void *fake_dlsym(void *loader, const char *name) {
  assert(loader == (void *)1 && strcmp(name, "vkGetInstanceProcAddr") == 0);
  return (void *)proc;
}

int fake_dlclose(void *loader) {
  assert(loader == (void *)1);
  close_calls++;
  return 0;
}

int main(void) {
  assert(tf_loader_open() == NULL);
  const char *paths[] = {
      "/opt/homebrew/lib/libvulkan.1.dylib",
      "/usr/local/lib/libvulkan.1.dylib",
      "/opt/homebrew/lib/libMoltenVK.dylib",
      "/usr/local/lib/libMoltenVK.dylib",
      "/opt/homebrew/opt/molten-vk/lib/libMoltenVK.dylib",
      "/usr/local/opt/molten-vk/lib/libMoltenVK.dylib",
  };
  for (size_t i = 0; i < sizeof(paths) / sizeof(paths[0]); i++) {
    available = paths[i];
    void *loader = tf_loader_open();
    assert(loader != NULL && tf_loader_proc(loader) == proc);
    tf_loader_close(loader);
  }
  assert(close_calls == 6);
  return 0;
}
