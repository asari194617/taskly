#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location(
    "prepare_reconcile", HERE / "prepare-reconcile-baseline.py"
)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


def proof():
    return {
        "version": 1,
        "target_sha": "2" * 40,
        "baseline_sha": "1" * 40,
        "database_digest": "3" * 64,
        "sources": {
            "apps/server/schema.sql": "4" * 64,
            "atlas.hcl": "5" * 64,
        },
    }


class PrepareTests(unittest.TestCase):
    def test_valid_exact_proof(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "baseline.json"
            path.write_text(json.dumps(proof()))
            self.assertEqual(m.validate(path), proof())

    def test_malformed_or_untrusted_proof(self):
        cases = [
            b"",
            b"not-json",
            b'{"version":1,"version":1}',
            json.dumps({**proof(), "extra": True}).encode(),
            json.dumps({**proof(), "version": True}).encode(),
            json.dumps({**proof(), "target_sha": "main"}).encode(),
            json.dumps({**proof(), "baseline_sha": "a" * 39}).encode(),
            json.dumps({**proof(), "database_digest": "b" * 63}).encode(),
            json.dumps({**proof(), "sources": {}}).encode(),
        ]
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "baseline.json"
            for payload in cases:
                path.write_bytes(payload)
                with self.subTest(payload=payload[:30]), self.assertRaises(SystemExit):
                    m.validate(path)

    def test_symlink_and_oversized_proof_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            target = root / "target"
            target.write_text(json.dumps(proof()))
            link = root / "link"
            link.symlink_to(target)
            with self.assertRaises(SystemExit):
                m.validate(link)
            target.write_bytes(b"x" * 4097)
            with self.assertRaises(SystemExit):
                m.validate(target)


if __name__ == "__main__":
    unittest.main()
