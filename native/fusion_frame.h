#ifndef PGCORE_FUSION_FRAME_H
#define PGCORE_FUSION_FRAME_H
#include <stddef.h>
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
#define PGFUSION_VERSION 1u
#define PGFUSION_HEADER_SIZE 16u
#define PGFUSION_MAX_PAYLOAD (64u * 1024u)
enum pgfusion_frame_type { PGFUSION_DATA=1, PGFUSION_PING=2, PGFUSION_PONG=3, PGFUSION_CLOSE=4 };
struct pgfusion_frame_view { uint8_t type; uint16_t flags; uint32_t sequence; const uint8_t *payload; uint32_t payload_len; };
/* Parse exactly one frame. Payload is a borrowed view into input. */
int pgfusion_frame_parse(const uint8_t *input, size_t input_len, struct pgfusion_frame_view *out);
/* Encode into caller-owned output; returns bytes written or 0 on error. */
size_t pgfusion_frame_encode(uint8_t type, uint16_t flags, uint32_t sequence,
 const uint8_t *payload, size_t payload_len, uint8_t *output, size_t output_capacity);
#ifdef __cplusplus
}
#endif
#endif
