#include "fusion_frame.h"

#include <string.h>

static uint16_t read_u16(const uint8_t *p) {
    return (uint16_t)(((uint16_t)p[0] << 8) | (uint16_t)p[1]);
}

static uint32_t read_u32(const uint8_t *p) {
    return ((uint32_t)p[0] << 24) | ((uint32_t)p[1] << 16) |
           ((uint32_t)p[2] << 8) | (uint32_t)p[3];
}

static void write_u16(uint8_t *p, uint16_t v) {
    p[0] = (uint8_t)(v >> 8);
    p[1] = (uint8_t)v;
}

static void write_u32(uint8_t *p, uint32_t v) {
    p[0] = (uint8_t)(v >> 24);
    p[1] = (uint8_t)(v >> 16);
    p[2] = (uint8_t)(v >> 8);
    p[3] = (uint8_t)v;
}

static int valid_type(uint8_t type) {
    return type >= PGFUSION_DATA && type <= PGFUSION_CLOSE;
}

int pgfusion_frame_parse(const uint8_t *input, size_t input_len, struct pgfusion_frame_view *out) {
    uint32_t payload_len;

    if (input == NULL || out == NULL || input_len < PGFUSION_HEADER_SIZE) {
        return -1;
    }
    if (memcmp(input, "PGF1", 4) != 0 || input[4] != PGFUSION_VERSION || !valid_type(input[5])) {
        return -1;
    }
    payload_len = read_u32(input + 12);
    if (payload_len > PGFUSION_MAX_PAYLOAD ||
        input_len != PGFUSION_HEADER_SIZE + (size_t)payload_len) {
        return -1;
    }

    out->type = input[5];
    out->flags = read_u16(input + 6);
    out->sequence = read_u32(input + 8);
    out->payload = input + PGFUSION_HEADER_SIZE;
    out->payload_len = payload_len;
    return 0;
}

size_t pgfusion_frame_encode(uint8_t type, uint16_t flags, uint32_t sequence,
                             const uint8_t *payload, size_t payload_len,
                             uint8_t *output, size_t output_capacity) {
    size_t total;

    if (!valid_type(type) || payload_len > PGFUSION_MAX_PAYLOAD ||
        (payload_len > 0 && payload == NULL) || output == NULL) {
        return 0;
    }
    total = PGFUSION_HEADER_SIZE + payload_len;
    if (output_capacity < total) {
        return 0;
    }

    memcpy(output, "PGF1", 4);
    output[4] = PGFUSION_VERSION;
    output[5] = type;
    write_u16(output + 6, flags);
    write_u32(output + 8, sequence);
    write_u32(output + 12, (uint32_t)payload_len);
    if (payload_len > 0) {
        memcpy(output + PGFUSION_HEADER_SIZE, payload, payload_len);
    }
    return total;
}
