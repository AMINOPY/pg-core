#ifndef PG_CORE_CDN_VALIDATION_H
#define PG_CORE_CDN_VALIDATION_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define PG_CORE_NATIVE_ABI_VERSION 1u

/* Returns the ABI version implemented by this library. */
uint32_t pg_core_native_abi_version(void);

/*
 * Validates a DNS hostname supplied as an SNI value.
 * Input is length-delimited and need not be NUL-terminated.
 * Returns 1 for valid input and 0 otherwise.
 * IP literals, empty labels, trailing dots, whitespace, and overlong names
 * are rejected. This is syntax validation, not proof of domain ownership.
 */
int pg_core_validate_sni(const char *value, size_t length);

#ifdef __cplusplus
}
#endif
#endif
