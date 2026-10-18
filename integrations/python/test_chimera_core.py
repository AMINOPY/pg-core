import json
import unittest

from chimera_core import ChimeraCoreClient, ChimeraCoreError


class FakeTransport:
    def __init__(self, response='{"switch":false,"current_id":"a"}'):
        self.response = response
        self.calls = []

    def __call__(self, method, payload):
        self.calls.append((method, payload))
        return self.response


class ChimeraCoreClientTests(unittest.TestCase):
    def test_evaluate_serializes_sample_as_json_string(self):
        transport = FakeTransport()
        client = ChimeraCoreClient(transport)
        sample = {
            "current_id": "a",
            "candidates": [
                {"id": "a", "subscription_id": "s", "status": "healthy", "latency_ms": 80}
            ],
        }
        decision = client.evaluate(sample)
        self.assertFalse(decision["switch"])
        method, payload = transport.calls[0]
        self.assertEqual(method, "evaluate")
        self.assertEqual(json.loads(payload["sample"]), sample)

    def test_rejects_malformed_or_wrong_shaped_decisions(self):
        for response in ("not-json", "[]", '{"switch":"yes"}'):
            with self.subTest(response=response):
                client = ChimeraCoreClient(FakeTransport(response))
                with self.assertRaises(ChimeraCoreError):
                    client.evaluate({"current_id": "a"})

    def test_switch_requires_target_id(self):
        client = ChimeraCoreClient(FakeTransport('{"switch":true}'))
        with self.assertRaises(ChimeraCoreError):
            client.evaluate({"current_id": "a"})

    def test_confirm_and_cancel_use_method_channel_names(self):
        transport = FakeTransport()
        client = ChimeraCoreClient(transport)
        client.confirm_switch("profile-2")
        client.cancel_switch()
        self.assertEqual(transport.calls[0], ("confirmSwitch", {"targetId": "profile-2"}))
        self.assertEqual(transport.calls[1], ("cancelSwitch", {}))

    def test_rejects_empty_target_id(self):
        client = ChimeraCoreClient(FakeTransport())
        with self.assertRaises(ValueError):
            client.confirm_switch("  ")


if __name__ == "__main__":
    unittest.main()
