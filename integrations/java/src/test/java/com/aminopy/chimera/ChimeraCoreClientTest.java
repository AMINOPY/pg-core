package com.aminopy.chimera;

import static org.junit.jupiter.api.Assertions.*;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class ChimeraCoreClientTest {
    private static final class Call {
        final String method;
        final Map<String, Object> payload;
        Call(String method, Map<String, Object> payload) {
            this.method = method;
            this.payload = payload;
        }
    }

    @Test
    void forwardsEvaluateConfirmAndCancel() throws Exception {
        List<Call> calls = new ArrayList<>();
        ChimeraCoreClient client = new ChimeraCoreClient((method, payload) -> {
            calls.add(new Call(method, payload));
            return "{\"switch\":false}";
        });

        assertEquals("{\"switch\":false}", client.evaluate("{\"current_id\":\"a\"}"));
        client.confirmSwitch("profile-2");
        client.cancelSwitch();

        assertEquals("evaluate", calls.get(0).method);
        assertEquals("{\"current_id\":\"a\"}", calls.get(0).payload.get("sample"));
        assertEquals("confirmSwitch", calls.get(1).method);
        assertEquals("profile-2", calls.get(1).payload.get("targetId"));
        assertEquals("cancelSwitch", calls.get(2).method);
    }

    @Test
    void rejectsEmptyInputs() {
        ChimeraCoreClient client = new ChimeraCoreClient((method, payload) -> "");
        assertThrows(IllegalArgumentException.class, () -> client.evaluate(" "));
        assertThrows(IllegalArgumentException.class, () -> client.confirmSwitch(""));
    }

    @Test
    void requiresBridge() {
        assertThrows(NullPointerException.class, () -> new ChimeraCoreClient(null));
    }
}
