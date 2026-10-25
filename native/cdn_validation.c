#include "cdn_validation.h"

#include <stddef.h>

uint32_t pg_core_native_abi_version(void) {
    return PG_CORE_NATIVE_ABI_VERSION;
}

static int is_ascii_alnum(unsigned char c) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
           (c >= '0' && c <= '9');
}

/* Return true only for a complete dotted-decimal IPv4 literal. */
static int is_ipv4_literal(const char *value, size_t length) {
    size_t i = 0;
    unsigned int parts = 0;

    while (i < length) {
        unsigned int octet = 0;
        size_t digits = 0;

        while (i < length && value[i] >= '0' && value[i] <= '9') {
            octet = octet * 10u + (unsigned int)(value[i] - '0');
            ++digits;
            ++i;
            if (octet > 255u || digits > 3u) {
                return 0;
            }
        }

        if (digits == 0) {
            return 0;
        }
        ++parts;

        if (i == length) {
            break;
        }
        if (value[i] != '.') {
            return 0;
        }
        ++i;
        if (i == length) {
            return 0;
        }
    }

    return parts == 4u;
}

int pg_core_validate_sni(const char *value, size_t length) {
    size_t label_length = 0;
    size_t i;

    if (value == NULL || length == 0 || length > 253) {
        return 0;
    }

    /* SNI here must be an ASCII DNS hostname, not an IP literal. */
    if (is_ipv4_literal(value, length)) {
        return 0;
    }

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

        if (!(is_ascii_alnum(c) || c == '-') ||
            (label_length == 0 && c == '-')) {
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
