#include "ghost_plus.h"
#include <assert.h>
int main(void) {
    pg_ghost_observation healthy = {3U,3U,3U,0U,0U,0U,0U,0U};
    pg_ghost_observation dns_fail = {3U,0U,0U,0U,0U,0U,0U,0U};
    pg_ghost_observation denied = {3U,3U,1U,0U,0U,0U,1U,0U};
    pg_ghost_observation invalid = {21U,21U,21U,0U,0U,0U,0U,0U};
    assert(pg_ghost_plus_analyze(&healthy).status == PG_GHOST_HEALTHY);
    assert(pg_ghost_plus_analyze(&dns_fail).status == PG_GHOST_DNS_FAILURE);
    assert(pg_ghost_plus_analyze(&denied).status == PG_GHOST_POSSIBLE_FILTERING);
    assert(pg_ghost_plus_analyze(&invalid).status == PG_GHOST_INCONCLUSIVE);
    assert(pg_ghost_plus_analyze(0).status == PG_GHOST_INCONCLUSIVE);
    return 0;
}
