#define VK_NO_PROTOTYPES
#include <assert.h>
#include <stdio.h>
#include <string.h>
#include <vulkan/vulkan.h>

#include "../compute.c"

static int portability, properties2, subset, software_only, layer_settings;
static const char *missing_device_function;
static int instance_created, device_created, instance_destroyed,
    device_destroyed;
static int loader_closed, device_queries;

static VKAPI_ATTR VkResult VKAPI_CALL instance_extensions(
    const char *layer, uint32_t *count, VkExtensionProperties *props) {
  assert(layer == NULL);
  if (props) {
    if (portability)
      strcpy(props[0].extensionName, "VK_KHR_portability_enumeration");
    if (properties2)
      strcpy(props[portability].extensionName,
             "VK_KHR_get_physical_device_properties2");
    if (layer_settings)
      strcpy(props[portability + properties2].extensionName,
             "VK_EXT_layer_settings");
  }
  *count = portability + properties2 + layer_settings;
  return VK_SUCCESS;
}

static VKAPI_ATTR VkResult VKAPI_CALL
create_instance(const VkInstanceCreateInfo *info,
                const VkAllocationCallbacks *alloc, VkInstance *instance) {
  assert(alloc == NULL);
  uint32_t expected_extensions = portability + properties2;
#ifdef __APPLE__
  expected_extensions += layer_settings;
#endif
  assert(info->enabledExtensionCount == expected_extensions);
#ifdef __APPLE__
  if (layer_settings) {
    assert(strcmp(info->ppEnabledExtensionNames[portability + properties2],
                  "VK_EXT_layer_settings") == 0);
    const VkLayerSettingsCreateInfoEXT *settings = info->pNext;
    assert(settings && settings->sType == VK_STRUCTURE_TYPE_LAYER_SETTINGS_CREATE_INFO_EXT);
    assert(settings->pNext == NULL && settings->settingCount == 1);
    const VkLayerSettingEXT *setting = settings->pSettings;
    assert(strcmp(setting->pLayerName, "MoltenVK") == 0);
    assert(strcmp(setting->pSettingName, "MVK_CONFIG_USE_METAL_ARGUMENT_BUFFERS") == 0);
    assert(setting->type == VK_LAYER_SETTING_TYPE_BOOL32_EXT && setting->valueCount == 1);
    assert(*(const VkBool32 *)setting->pValues == VK_FALSE);
  } else
#endif
    assert(info->pNext == NULL);
  if (portability)
    assert(strcmp(info->ppEnabledExtensionNames[0],
                  "VK_KHR_portability_enumeration") == 0);
  if (properties2)
    assert(strcmp(info->ppEnabledExtensionNames[portability],
                  "VK_KHR_get_physical_device_properties2") == 0);
  assert(info->flags ==
         (portability ? VK_INSTANCE_CREATE_ENUMERATE_PORTABILITY_BIT_KHR : 0));
  *instance = (VkInstance)(uintptr_t)1;
  instance_created++;
  return VK_SUCCESS;
}

static VKAPI_ATTR void VKAPI_CALL
destroy_instance(VkInstance instance, const VkAllocationCallbacks *alloc) {
  assert(instance == (VkInstance)(uintptr_t)1 && alloc == NULL);
  instance_destroyed++;
}

static VKAPI_ATTR VkResult VKAPI_CALL physical_devices(
    VkInstance instance, uint32_t *count, VkPhysicalDevice *devices) {
  assert(instance == (VkInstance)(uintptr_t)1);
  *count = 1;
  if (devices)
    devices[0] = (VkPhysicalDevice)(uintptr_t)2;
  return VK_SUCCESS;
}

static VKAPI_ATTR void VKAPI_CALL physical_properties(
    VkPhysicalDevice physical, VkPhysicalDeviceProperties *props) {
  assert(physical == (VkPhysicalDevice)(uintptr_t)2);
  memset(props, 0, sizeof(*props));
  props->deviceType = software_only ? VK_PHYSICAL_DEVICE_TYPE_CPU
                                    : VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU;
  props->limits.minStorageBufferOffsetAlignment = 16;
}

static VKAPI_ATTR void VKAPI_CALL
queue_properties(VkPhysicalDevice physical, uint32_t *count,
                 VkQueueFamilyProperties *props) {
  assert(physical == (VkPhysicalDevice)(uintptr_t)2);
  *count = 1;
  if (props) {
    memset(props, 0, sizeof(*props));
    props->queueFlags = VK_QUEUE_COMPUTE_BIT;
    props->queueCount = 1;
  }
}

static VKAPI_ATTR VkResult VKAPI_CALL
device_extensions(VkPhysicalDevice physical, const char *layer, uint32_t *count,
                  VkExtensionProperties *props) {
  assert(physical == (VkPhysicalDevice)(uintptr_t)2 && layer == NULL);
  *count = subset ? 1 : 0;
  if (props && subset)
    strcpy(props[0].extensionName, "VK_KHR_portability_subset");
  return VK_SUCCESS;
}

static VKAPI_ATTR VkResult VKAPI_CALL
create_device(VkPhysicalDevice physical, const VkDeviceCreateInfo *info,
              const VkAllocationCallbacks *alloc, VkDevice *device) {
  assert(physical == (VkPhysicalDevice)(uintptr_t)2 && alloc == NULL);
  assert(info->enabledExtensionCount == (subset ? 1u : 0u));
  if (subset)
    assert(strcmp(info->ppEnabledExtensionNames[0],
                  "VK_KHR_portability_subset") == 0);
  *device = (VkDevice)(uintptr_t)3;
  device_created++;
  return VK_SUCCESS;
}

static VKAPI_ATTR void VKAPI_CALL
destroy_device(VkDevice device, const VkAllocationCallbacks *alloc) {
  assert(device == (VkDevice)(uintptr_t)3 && alloc == NULL);
  device_destroyed++;
}

static VKAPI_ATTR VkResult VKAPI_CALL wait_idle(VkDevice device) {
  assert(device == (VkDevice)(uintptr_t)3);
  return VK_SUCCESS;
}

static VKAPI_ATTR void VKAPI_CALL get_queue(VkDevice device, uint32_t family,
                                            uint32_t index, VkQueue *queue) {
  assert(device == (VkDevice)(uintptr_t)3 && family == 0 && index == 0);
  *queue = (VkQueue)(uintptr_t)4;
}

static VKAPI_ATTR VkResult VKAPI_CALL
fail_layout(VkDevice device, const VkDescriptorSetLayoutCreateInfo *info,
            const VkAllocationCallbacks *alloc, VkDescriptorSetLayout *layout) {
  assert(device == (VkDevice)(uintptr_t)3 && alloc == NULL);
  (void)info;
  (void)layout;
  return VK_ERROR_FEATURE_NOT_PRESENT;
}

static VKAPI_ATTR void VKAPI_CALL unused(void) {
  assert(!"unexpected Vulkan call");
}

static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL device_proc(VkDevice device,
                                                            const char *name) {
  assert(device == (VkDevice)(uintptr_t)3);
  device_queries++;
  if (missing_device_function && strcmp(name, missing_device_function) == 0)
    return NULL;
  if (strcmp(name, "vkDestroyDevice") == 0)
    return (PFN_vkVoidFunction)destroy_device;
  if (strcmp(name, "vkDeviceWaitIdle") == 0)
    return (PFN_vkVoidFunction)wait_idle;
  if (strcmp(name, "vkGetDeviceQueue") == 0)
    return (PFN_vkVoidFunction)get_queue;
  if (strcmp(name, "vkCreateDescriptorSetLayout") == 0)
    return (PFN_vkVoidFunction)fail_layout;
  return unused;
}

static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL
instance_proc(VkInstance instance, const char *name) {
  if (strcmp(name, "vkCreateInstance") == 0)
    return (PFN_vkVoidFunction)create_instance;
  if (strcmp(name, "vkEnumerateInstanceExtensionProperties") == 0)
    return (PFN_vkVoidFunction)instance_extensions;
  assert(instance == (VkInstance)(uintptr_t)1);
  if (strcmp(name, "vkDestroyInstance") == 0)
    return (PFN_vkVoidFunction)destroy_instance;
  if (strcmp(name, "vkEnumeratePhysicalDevices") == 0)
    return (PFN_vkVoidFunction)physical_devices;
  if (strcmp(name, "vkGetPhysicalDeviceProperties") == 0)
    return (PFN_vkVoidFunction)physical_properties;
  if (strcmp(name, "vkGetPhysicalDeviceQueueFamilyProperties") == 0)
    return (PFN_vkVoidFunction)queue_properties;
  if (strcmp(name, "vkEnumerateDeviceExtensionProperties") == 0)
    return (PFN_vkVoidFunction)device_extensions;
  if (strcmp(name, "vkCreateDevice") == 0)
    return (PFN_vkVoidFunction)create_device;
  if (strcmp(name, "vkGetDeviceProcAddr") == 0)
    return (PFN_vkVoidFunction)device_proc;
  if (strcmp(name, "vkDestroyDevice") == 0)
    return (PFN_vkVoidFunction)destroy_device;
  return unused;
}

void *tf_loader_open(void) { return (void *)1; }
PFN_vkGetInstanceProcAddr tf_loader_proc(void *loader) {
  assert(loader == (void *)1);
  return instance_proc;
}
void tf_loader_close(void *loader) {
  assert(loader == (void *)1);
  loader_closed++;
}

int main(void) {
  struct {
    int enumeration, properties, subset, software, creates_device;
    const char *missing, *error;
  } cases[] = {
      {0, 0, 0, 0, 1, NULL, "VkResult"},
      {1, 1, 1, 0, 1, NULL, "VkResult"},
      {0, 1, 1, 0, 1, NULL, "VkResult"},
      {1, 0, 1, 0, 0, NULL, "requires VK_KHR_get_physical_device_properties2"},
      {0, 0, 0, 0, 1, "vkCmdDispatch", "missing vkCmdDispatch"},
      {0, 0, 0, 0, 1, "vkDeviceWaitIdle", "missing vkDeviceWaitIdle"},
      {0, 0, 0, 0, 1, "vkDestroyDevice", "missing vkDestroyDevice"},
      {0, 0, 0, 1, 0, NULL, "no hardware"},
  };
  for (size_t i = 0; i < sizeof(cases) / sizeof(cases[0]); i++) {
    portability = cases[i].enumeration;
    properties2 = cases[i].properties;
    subset = cases[i].subset;
    layer_settings = subset && properties2;
#ifdef __APPLE__
    layer_settings = 1;
#endif
    missing_device_function = cases[i].missing;
    software_only = cases[i].software;
    instance_created = device_created = instance_destroyed = device_destroyed =
        loader_closed = device_queries = 0;
    uint32_t code[5] = {0x07230203};
    char error[256] = {0};
    tf_gpu *gpu = tf_gpu_open(code, sizeof(code), error, sizeof(error));
    assert(gpu == NULL);
    if (instance_created != 1) {
      fprintf(stderr,
              "loader must resolve Vulkan entry points via "
              "vkGetInstanceProcAddr: %s\n",
              error);
      return 1;
    }
    assert(instance_destroyed == 1 && loader_closed == 1);
    assert(device_created == cases[i].creates_device &&
           device_destroyed == cases[i].creates_device);
    assert(strstr(error, cases[i].error) != NULL);
    if (cases[i].creates_device)
      assert(device_queries > 0);
  }
#ifdef __APPLE__
  layer_settings = 0;
  instance_created = loader_closed = 0;
  uint32_t code[5] = {0x07230203};
  char error[256] = {0};
  assert(tf_gpu_open(code, sizeof(code), error, sizeof(error)) == NULL);
  assert(strstr(error, "requires VK_EXT_layer_settings") != NULL);
  assert(instance_created == 0 && loader_closed == 1);
#endif
  return 0;
}
