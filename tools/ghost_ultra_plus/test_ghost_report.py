import unittest
from ghost_report import summarize

class GhostReportTest(unittest.TestCase):
    def test_english_report(self):
        text = summarize({"domain":"example.org","samples":2,"dns_successes":2,"https_successes":1})
        self.assertIn("Ghost report", text)
        self.assertIn("do not prove DPI", text)
    def test_persian_report(self):
        text = summarize({"domain":"example.org","samples":1,"dns_successes":1,"https_successes":1}, "fa")
        self.assertIn("گزارش شبح", text)
        self.assertIn("اثبات نمی‌کنند", text)
    def test_rejects_out_of_bounds(self):
        with self.assertRaises(ValueError):
            summarize({"samples":21,"dns_successes":0,"https_successes":0})
    def test_rejects_unknown_language(self):
        with self.assertRaises(ValueError):
            summarize({"samples":0,"dns_successes":0,"https_successes":0}, "xx")

if __name__ == "__main__":
    unittest.main()
