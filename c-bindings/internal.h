#pragma once

#include <stdint.h>
// extern "C" {
//     #include "libjsonnet.h"
// }

typedef struct JsonnetJsonValue *JsonnetNativeCallback(void *ctx,
                                                       const struct JsonnetJsonValue *const *argv,
                                                       int *success);

struct JsonnetVm {
    uint32_t id;
};

struct JsonnetVm *jsonnet_internal_make_vm_with_id(uint32_t id);
void jsonnet_internal_free_vm(struct JsonnetVm *x);

// jsonnet_internal_call_callback, because calling C function pointers from Go is not supported
inline struct JsonnetJsonValue *jsonnet_internal_call_callback(JsonnetNativeCallback cb,
                                                               void *ctx,
                                                               const struct JsonnetJsonValue *const *argv,
                                                               int *success) {
    return cb(ctx, argv, success);
}

typedef struct JsonnetJsonValue *JsonnetNativeCallback(void *ctx,
                                                       const struct JsonnetJsonValue *const *argv,
                                                       int *success);
