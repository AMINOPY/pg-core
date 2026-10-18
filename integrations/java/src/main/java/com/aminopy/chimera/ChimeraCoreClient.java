package com.aminopy.chimera;

import java.util.Map;
import java.util.Objects;

/**
 * Small Java facade for a host-provided Chimera JSON bridge.
 *
 * This class intentionally does not guess generated gomobile class names or
 * own VPN/network routing. The application supplies a Bridge implementation.
 */
public final class ChimeraCoreClient {
    public interface Bridge {
        String invoke(String method, Map<String, Object> payload) throws Exception;
    }

    private final Bridge bridge;

    public ChimeraCoreClient(Bridge bridge) {
        this.bridge = Objects.requireNonNull(bridge, "bridge");
    }

    public String evaluate(String sampleJson) throws Exception {
        if (sampleJson == null || sampleJson.trim().isEmpty()) {
            throw new IllegalArgumentException("sampleJson must not be empty");
        }
        return bridge.invoke("evaluate", java.util.Collections.<String, Object>singletonMap(
                "sample", sampleJson));
    }

    public void confirmSwitch(String targetId) throws Exception {
        requireId(targetId);
        bridge.invoke("confirmSwitch", java.util.Collections.<String, Object>singletonMap(
                "targetId", targetId));
    }

    public void cancelSwitch() throws Exception {
        bridge.invoke("cancelSwitch", java.util.Collections.<String, Object>emptyMap());
    }

    private static void requireId(String targetId) {
        if (targetId == null || targetId.trim().isEmpty()) {
            throw new IllegalArgumentException("targetId must not be empty");
        }
    }
}
