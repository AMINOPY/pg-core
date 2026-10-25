#include "cdn_validation.h"

#include <assert.h>
#include <stddef.h>
#include <string.h>

int main(void) {
    static const char good[] = "cdn.example.com";
    static const char hyphen[] = "edge-1.example.net";
    static const char empty[] = "";
    static const char bad_start[] = "-edge.example";
    static const char bad_end[] = "edge-.example";
    static const char bad_label[] = "edge..example";
    static const char whitespace[] = "edge example";
    static const char ip_literal[] = "192.0.2.1";
    static const char trailing_dot[] = "cdn.example.";
    static const char underscore[] = "cdn_edge.example";
    static const char ipv6_literal[] = "2001:db8::1";
    static const char tab[] = "cdn\texample.com";
    static const char long_label[] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example";

    assert(pg_core_native_abi_version() == PG_CORE_NATIVE_ABI_VERSION);
    assert(pg_core_validate_sni(good, sizeof(good) - 1) == 1);
    assert(pg_core_validate_sni(hyphen, sizeof(hyphen) - 1) == 1);
    assert(pg_core_validate_sni(empty, 0) == 0);
    assert(pg_core_validate_sni(NULL, 1) == 0);
    assert(pg_core_validate_sni(bad_start, sizeof(bad_start) - 1) == 0);
    assert(pg_core_validate_sni(bad_end, sizeof(bad_end) - 1) == 0);
    assert(pg_core_validate_sni(bad_label, sizeof(bad_label) - 1) == 0);
    assert(pg_core_validate_sni(whitespace, sizeof(whitespace) - 1) == 0);
    assert(pg_core_validate_sni(ip_literal, sizeof(ip_literal) - 1) == 0);
    assert(pg_core_validate_sni(trailing_dot, sizeof(trailing_dot) - 1) == 0);
    assert(pg_core_validate_sni(underscore, sizeof(underscore) - 1) == 0);
    assert(pg_core_validate_sni(ipv6_literal, sizeof(ipv6_literal) - 1) == 0);
    assert(pg_core_validate_sni(tab, sizeof(tab) - 1) == 0);
    assert(pg_core_validate_sni(long_label, sizeof(long_label) - 1) == 0);
    assert(pg_core_validate_sni(good, 254) == 0);
    return 0;
}
