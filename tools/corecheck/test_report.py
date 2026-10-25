import json
import tempfile
import unittest
from pathlib import Path
from report import load_report, render

class ReportTests(unittest.TestCase):
    def make_report(self, value):
        handle = tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', delete=False)
        with handle:
            json.dump(value, handle)
        path = Path(handle.name)
        self.addCleanup(path.unlink, missing_ok=True)
        return path

    def test_valid_report(self):
        path = self.make_report({'benchmarks': [{'name': 'lookup', 'duration_ns': 2, 'iterations': 10}]})
        self.assertIn('lookup | 2 | 10', render(load_report(path)))

    def test_empty_report_rejected(self):
        with self.assertRaises(ValueError):
            load_report(self.make_report({'benchmarks': []}))

    def test_negative_duration_rejected(self):
        path = self.make_report({'benchmarks': [{'name': 'x', 'duration_ns': -1, 'iterations': 1}]})
        with self.assertRaises(ValueError):
            load_report(path)

if __name__ == '__main__':
    unittest.main()
