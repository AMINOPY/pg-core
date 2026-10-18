package com.aminopy.chimera

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class ChimeraCoreClientTest {
    @Test
    fun forwardsEvaluateConfirmAndCancel() {
        val calls = mutableListOf<Pair<String, Map<String, Any?>>>()
        val client = ChimeraCoreClient { method, payload ->
            calls += method to payload
            """{"switch":false}"""
        }

        assertEquals("""{"switch":false}""", client.evaluate("""{"current_id":"a"}"""))
        client.confirmSwitch("profile-2")
        client.cancelSwitch()

        assertEquals("evaluate", calls[0].first)
        assertEquals("""{"current_id":"a"}""", calls[0].second["sample"])
        assertEquals("confirmSwitch", calls[1].first)
        assertEquals("profile-2", calls[1].second["targetId"])
        assertEquals("cancelSwitch", calls[2].first)
    }

    @Test
    fun rejectsBlankInputs() {
        val client = ChimeraCoreClient { _, _ -> "" }
        assertFailsWith<IllegalArgumentException> { client.evaluate(" ") }
        assertFailsWith<IllegalArgumentException> { client.confirmSwitch(" ") }
    }
}
