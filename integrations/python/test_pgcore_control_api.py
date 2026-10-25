import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from pgcore_control_api import PGCoreAPIError, PGCoreControlClient


KEY = "test-key-" + "x" * 40


class Handler(BaseHTTPRequestHandler):
    last_auth = None
    last_body = None
    response_status = 200
    response_body = {"app_id": "demo-app", "api_version": "v1"}
    redirect = False

    def do_GET(self):
        type(self).last_auth = self.headers.get("Authorization")
        if type(self).redirect:
            self.send_response(302)
            self.send_header("Location", "https://example.invalid/collect")
            self.end_headers()
            return
        self.send_response(type(self).response_status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(type(self).response_body).encode())

    def do_POST(self):
        type(self).last_auth = self.headers.get("Authorization")
        type(self).last_body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.send_response(type(self).response_status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(type(self).response_body).encode())

    def do_PUT(self):
        self.do_POST()

    def log_message(self, *_):
        pass


class ClientTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.base_url = f"http://127.0.0.1:{cls.server.server_port}"

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(timeout=2)

    def setUp(self):
        Handler.last_auth = None
        Handler.last_body = None
        Handler.response_status = 200
        Handler.response_body = {"app_id": "demo-app", "api_version": "v1"}
        Handler.redirect = False
        self.client = PGCoreControlClient(self.base_url, KEY)

    def test_read_uses_bearer_key_and_returns_json(self):
        self.assertEqual(self.client.get_app()["app_id"], "demo-app")
        self.assertEqual(Handler.last_auth, "Bearer " + KEY)

    def test_admin_request_serializes_json(self):
        self.client.publish_announcement("Maintenance tonight")
        self.assertEqual(Handler.last_body, {"message": "Maintenance tonight"})

    def test_user_id_and_status_are_validated_before_network(self):
        with self.assertRaises(ValueError):
            self.client.get_user_status("../other-app")
        with self.assertRaises(ValueError):
            self.client.set_user_status("user-1", "owner")

    def test_only_android_and_windows_are_supported_by_this_sdk(self):
        with self.assertRaises(ValueError):
            self.client.get_latest_release("linux")

    def test_requires_https_except_loopback_development(self):
        with self.assertRaises(ValueError):
            PGCoreControlClient("http://example.com", KEY)
        with self.assertRaises(ValueError):
            PGCoreControlClient("https://example.com", "short")

    def test_http_error_exposes_stable_code_not_body(self):
        Handler.response_status = 403
        Handler.response_body = {"error": "insufficient_scope", "private": "must not leak"}
        with self.assertRaises(PGCoreAPIError) as caught:
            self.client.publish_announcement("hello")
        self.assertEqual(caught.exception.status, 403)
        self.assertEqual(caught.exception.code, "insufficient_scope")
        self.assertNotIn("must not leak", str(caught.exception))

    def test_redirect_is_not_followed(self):
        Handler.redirect = True
        with self.assertRaises(PGCoreAPIError):
            self.client.get_app()


if __name__ == "__main__":
    unittest.main()
