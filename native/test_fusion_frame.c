#include "fusion_frame.h"

#include <assert.h>
#include <stdint.h>
#include <string.h>

int main(void) {
    const uint8_t payload[] = {'h', 'i'};
    uint8_t wire[PGFUSION_HEADER_SIZE + sizeof(payload)];
    struct pgfusion_frame_view view;
    const uint8_t golden[] = {
        0x50, 0x47, 0x46, 0x31, 0x01, 0x01, 0x00, 0x07,
        0x10, 0x20, 0x30, 0x40, 0x00, 0x00, 0x00, 0x02, 0x68, 0x69
    };
    size_t n = pgfusion_frame_encode(PGFUSION_DATA, 7, 0x10203040u,
                                     payload, sizeof(payload), wire, sizeof(wire));

    assert(n == sizeof(wire));
    assert(memcmp(wire, golden, sizeof(golden)) == 0);
    assert(pgfusion_frame_parse(wire, n, &view) == 0);
    assert(view.type == PGFUSION_DATA && view.flags == 7);
    assert(view.sequence == 0x10203040u && view.payload_len == sizeof(payload));
    assert(memcmp(view.payload, payload, sizeof(payload)) == 0);
    assert(pgfusion_frame_parse(wire, n - 1, &view) != 0);
    wire[4] = 99;
    assert(pgfusion_frame_parse(wire, n, &view) != 0);
    assert(pgfusion_frame_encode(PGFUSION_DATA, 0, 0, payload, sizeof(payload),
                                 wire, PGFUSION_HEADER_SIZE) == 0);
    assert(pgfusion_frame_encode(PGFUSION_DATA, 0, 0, payload,
                                 PGFUSION_MAX_PAYLOAD + 1u, wire, sizeof(wire)) == 0);
    return 0;
}
