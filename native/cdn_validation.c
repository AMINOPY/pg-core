#include "cdn_validation.h"

#include <ctype.h>
#include <stddef.h>

uint32_t pg_core_native_abi_version(void) {
    return PG_CORE_NATIVE_ABI_VERSION;
}

int pg_core_validate_sni(const char *value, size_t length) {
    size_t label_length = 0;
    size_t i;

    if (value == NULL || length == 0 || length > 253) {
        return 0;
    }

    /* Require DNS hostname syntax; this intentionally excludes IP literals. */
    for (i = 0; i < length; ++i) {
        unsigned char c = (unsigned char)value[i];

        if (c == '.') {
            if (label_length == 0 || label_length > 63 ||
                value[i - 1] == '-') {
                return 0;
            }
            label_length = 0;
            continue;
        }

        if (!(isalnum(c) || c == '-') || (label_length == 0 && c == '-')) {
            return 0;
        }

        ++label_length;
        if (label_length > 63) {
            return 0;
        }
    }

    if (label_length == 0 || value[length - 1] == '-') {
        return 0;
    }

    return 1;
}
