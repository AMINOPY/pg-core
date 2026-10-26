#ifndef PG_CORE_GHOST_PLUS_H
#define PG_CORE_GHOST_PLUS_H
#ifdef __cplusplus
extern "C" {
#endif
typedef enum {
    PG_GHOST_HEALTHY = 0, PG_GHOST_DNS_FAILURE, PG_GHOST_HTTPS_UNREACHABLE,
    PG_GHOST_POSSIBLE_FILTERING, PG_GHOST_INCONCLUSIVE
} pg_ghost_status;
/* Ghost Plus classifies observations only; it never changes network state. */
typedef struct {
    unsigned int samples, dns_successes, https_successes, tls_errors;
    unsigned int connection_resets, timeouts, http_403, http_451;
} pg_ghost_observation;
typedef struct {
    pg_ghost_status status;
    unsigned int confidence_percent; /* heuristic, not DPI probability */
    const char *summary;
    const char *recommendation;
} pg_ghost_finding;
/* Samples are capped at 20. Invalid/empty input returns INCONCLUSIVE. */
pg_ghost_finding pg_ghost_plus_analyze(const pg_ghost_observation *observation);
#ifdef __cplusplus
}
#endif
#endif
