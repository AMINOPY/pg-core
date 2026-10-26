#include "ghost_plus.h"
pg_ghost_finding pg_ghost_plus_analyze(const pg_ghost_observation *o) {
    pg_ghost_finding f = { PG_GHOST_INCONCLUSIVE, 0U,
        "Insufficient or invalid observations to classify connectivity.",
        "Repeat a small test, verify hostname, device time, captive portal, and server status." };
    if (o == 0 || o->samples == 0U || o->samples > 20U ||
        o->dns_successes > o->samples || o->https_successes > o->samples ||
        o->tls_errors > o->samples || o->connection_resets > o->samples ||
        o->timeouts > o->samples || o->http_403 > o->samples || o->http_451 > o->samples)
        return f;
    if (o->https_successes == o->samples && o->dns_successes == o->samples) {
        f.status = PG_GHOST_HEALTHY; f.confidence_percent = 90U;
        f.summary = "DNS and HTTPS succeeded for all samples.";
        f.recommendation = "No action is indicated by these observations.";
        return f;
    }
    if (o->dns_successes == 0U) {
        f.status = PG_GHOST_DNS_FAILURE; f.confidence_percent = 75U;
        f.summary = "DNS resolution failed for all samples; cause is not established.";
        f.recommendation = "Check resolver settings, captive portal, hostname, and a trusted alternate network.";
        return f;
    }
    if (o->http_403 + o->http_451 > 0U) {
        f.status = PG_GHOST_POSSIBLE_FILTERING; f.confidence_percent = 55U;
        f.summary = "HTTP policy-denial responses observed; this alone does not prove DPI.";
        f.recommendation = "Verify server policy and endpoint availability before changing transports.";
        return f;
    }
    if (o->tls_errors + o->connection_resets + o->timeouts > 0U ||
        o->https_successes < o->samples) {
        f.status = PG_GHOST_HTTPS_UNREACHABLE; f.confidence_percent = 45U;
        f.summary = "HTTPS or transport failures observed; several causes are possible.";
        f.recommendation = "Check device time, certificate chain, server health, and a known-good endpoint.";
    }
    return f;
}
