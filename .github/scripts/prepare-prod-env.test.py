import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("prepare_prod_env", Path(__file__).with_name("prepare-prod-env.py"))
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class PrepareProdEnvTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.output = Path(self.temp.name) / "github-env"
        self.env = {
            "PROD_ENV": "AIRFLOW_META_PASSWORD=keep-this-secret\nEXISTING=unchanged\n",
            "GITHUB_ENV": str(self.output),
            **{"TASK_IMAGES_" + key: "private-" + key for key in prepare.STORAGE_KEYS},
        }

    def run_prepare(self):
        stdout = io.StringIO()
        with patch.dict(os.environ, self.env, clear=True), contextlib.redirect_stdout(stdout):
            prepare.main()
        self.assertEqual(stdout.getvalue(), "")

    def guidance(self, storage=False):
        if not storage:
            self.env = {k: v for k, v in self.env.items() if not k.startswith("TASK_IMAGES_")}
        bindings = {
            "assigned-visit-2": 101,
            "visit-survey-2": 102,
            "overall-survey": 103,
            "sheet-submission": 104,
        }
        self.env["DEPLOY_NNCT_GUIDANCE_EVENT_ID"] = "synthetic-event-2030"
        self.env["DEPLOY_NNCT_GUIDANCE_TASK_BINDINGS"] = json.dumps(bindings)
        return bindings

    def merged(self):
        text = self.output.read_text()
        first, rest = text.split("\n", 1)
        delimiter = first.removeprefix("PROD_ENV<<")
        self.assertTrue(rest.endswith(delimiter + "\n"))
        return rest[: -len(delimiter + "\n")]

    def test_overrides_only_storage_and_preserves_other_settings(self):
        self.env["PROD_ENV"] += "R2_BUCKET=public-cdn\nexport R2_ACCESS_KEY_ID=old\nR2_BUCKET=another-cdn\n"
        self.run_prepare()
        lines = self.output.read_text().splitlines()
        self.assertEqual(lines[0], "PROD_ENV<<" + lines[-1])
        merged = "\n".join(lines[1:-1])
        self.assertIn("AIRFLOW_META_PASSWORD=keep-this-secret", merged)
        self.assertIn("EXISTING=unchanged", merged)
        for key in prepare.STORAGE_KEYS:
            self.assertEqual(sum(line.startswith(key + "=") for line in lines), 1)
            self.assertIn(key + "=private-" + key, merged)
        self.assertNotIn("public-cdn", merged)
        self.assertNotIn("=old", merged)

    def test_no_dedicated_secrets_leaves_existing_environment_alone(self):
        self.env = {k: v for k, v in self.env.items() if not k.startswith("TASK_IMAGES_")}
        self.run_prepare()
        self.assertFalse(self.output.exists())

    def test_partial_dedicated_secrets_cannot_mix_credentials(self):
        del self.env["TASK_IMAGES_R2_SECRET_ACCESS_KEY"]
        with self.assertRaisesRegex(SystemExit, "TASK_IMAGES_R2_SECRET_ACCESS_KEY"):
            self.run_prepare()
        self.assertFalse(self.output.exists())

    def test_storage_secrets_cannot_inject_env_assignments(self):
        self.env["TASK_IMAGES_R2_BUCKET"] = "bucket\nINJECTED=value"
        with self.assertRaisesRegex(SystemExit, "TASK_IMAGES_R2_BUCKET"):
            self.run_prepare()
        self.assertFalse(self.output.exists())

    def test_storage_secrets_do_not_replace_missing_production_environment(self):
        self.env["PROD_ENV"] = "   "
        with self.assertRaisesRegex(SystemExit, "PROD_ENV must contain"):
            self.run_prepare()
        self.assertFalse(self.output.exists())

    def test_guidance_without_storage_preserves_complete_original_environment(self):
        bindings = self.guidance()
        original = "# private source\nAIRFLOW_META_PASSWORD=keep-$literal  \nUNKNOWN_SECRET='leave-as-is'\nR2_BUCKET=original\nNNCT_GUIDANCE_EVENT_ID_SUFFIX=unchanged\n\n"
        self.env["PROD_ENV"] = original + "NNCT_GUIDANCE_EVENT_ID=old\n export NNCT_GUIDANCE_TASK_BINDINGS =old\nNNCT_GUIDANCE_EVENT_ID=duplicate\n"
        self.run_prepare()
        merged = self.merged()
        self.assertTrue(merged.startswith(original))
        self.assertEqual(merged.count("\nNNCT_GUIDANCE_EVENT_ID="), 1)
        self.assertIn("NNCT_GUIDANCE_EVENT_ID=synthetic-event-2030\n", merged)
        expected = json.dumps(bindings, sort_keys=True, separators=(",", ":"))
        self.assertIn("NNCT_GUIDANCE_TASK_BINDINGS='" + expected + "'\n", merged)
        self.assertNotIn("=old", merged)

    def test_storage_and_guidance_merge_together(self):
        self.guidance(storage=True)
        self.run_prepare()
        merged = self.merged()
        for key in prepare.STORAGE_KEYS + prepare.GUIDANCE_KEYS:
            self.assertEqual(sum(line.startswith(key + "=") for line in merged.splitlines()), 1)
        self.assertIn("AIRFLOW_META_PASSWORD=keep-this-secret\n", merged)

    def test_empty_guidance_keeps_existing_guidance_when_storage_changes(self):
        original = "NNCT_GUIDANCE_EVENT_ID=existing\nNNCT_GUIDANCE_TASK_BINDINGS='existing-json'\n"
        self.env["PROD_ENV"] += original
        self.env.update({"DEPLOY_" + key: "" for key in prepare.GUIDANCE_KEYS})
        self.run_prepare()
        self.assertIn(original, self.merged())

    def test_empty_dedicated_groups_do_not_rewrite_original(self):
        self.env = {"PROD_ENV": "keep everything", "GITHUB_ENV": str(self.output)}
        self.env.update({"DEPLOY_" + key: "" for key in prepare.GUIDANCE_KEYS})
        self.run_prepare()
        self.assertFalse(self.output.exists())

    def test_guidance_requires_both_settings(self):
        for missing in prepare.GUIDANCE_KEYS:
            with self.subTest(missing=missing):
                self.guidance()
                del self.env["DEPLOY_" + missing]
                with self.assertRaisesRegex(SystemExit, "DEPLOY_" + missing):
                    self.run_prepare()
                self.assertFalse(self.output.exists())

    def test_dedicated_values_reject_cr_and_newlines_even_at_edges(self):
        self.guidance(storage=True)
        original = dict(self.env)
        for key in ["TASK_IMAGES_R2_BUCKET", "DEPLOY_NNCT_GUIDANCE_EVENT_ID", "DEPLOY_NNCT_GUIDANCE_TASK_BINDINGS"]:
            for suffix in ["\n", "\r", "\r\nINJECTED=value"]:
                with self.subTest(key=key, suffix=repr(suffix)):
                    self.env = dict(original)
                    self.env[key] += suffix
                    with self.assertRaisesRegex(SystemExit, key):
                        self.run_prepare()
                    self.assertFalse(self.output.exists())

    def test_guidance_rejects_invalid_ids_keys_and_json_without_printing_values(self):
        valid = self.guidance()
        invalid = ["not-json", "[]", "null", "x" * 16385]
        for value in [True, 1.5, "7", 0, -1, 2**64]:
            invalid.append(json.dumps({**valid, "overall-survey": value}))
        invalid.extend([
            json.dumps({**valid, "arbitrary-$KEY": 201}),
            json.dumps({**valid, "assigned-visit-0": 201}),
            json.dumps({**valid, "overall-survey": 101}),
            json.dumps({k: v for k, v in valid.items() if k != "overall-survey"}),
            json.dumps({k: v for k, v in valid.items() if not k.startswith("assigned-visit-")}),
            json.dumps({k: v for k, v in valid.items() if not k.startswith("visit-survey-")}),
            json.dumps({**{f"visit-survey-{n}": n for n in range(1, 64)}, "overall-survey": 101, "sheet-submission": 102}),
            '{"assigned-visit-1":1,"visit-survey-1":2,"overall-survey":3,"sheet-submission":4,"overall-survey":5}',
        ])
        for payload in invalid:
            with self.subTest(payload_type=type(payload).__name__):
                self.env["DEPLOY_NNCT_GUIDANCE_TASK_BINDINGS"] = payload
                with self.assertRaisesRegex(SystemExit, "must contain valid task bindings") as error:
                    self.run_prepare()
                self.assertNotIn(payload, str(error.exception))
                self.assertFalse(self.output.exists())

    def test_guidance_requires_both_visit_groups_even_with_four_keys(self):
        valid = self.guidance()
        for replace in ["assigned-visit-2", "visit-survey-2"]:
            bindings = dict(valid)
            del bindings[replace]
            bindings["visit-survey-3" if replace.startswith("assigned") else "assigned-visit-3"] = 105
            self.env["DEPLOY_NNCT_GUIDANCE_TASK_BINDINGS"] = json.dumps(bindings)
            with self.assertRaisesRegex(SystemExit, "valid task bindings"):
                self.run_prepare()

    def test_event_identifier_cannot_interpolate_or_break_dotenv(self):
        self.guidance()
        for event_id in ["$(anything)", "event'quoted", "event with spaces", "x" * 101, "event/2030"]:
            with self.subTest(event_id=event_id):
                self.env["DEPLOY_NNCT_GUIDANCE_EVENT_ID"] = event_id
                with self.assertRaisesRegex(SystemExit, "valid event identifier"):
                    self.run_prepare()
                self.assertFalse(self.output.exists())

    def test_guidance_does_not_replace_missing_complete_environment(self):
        self.guidance()
        self.env["PROD_ENV"] = ""
        with self.assertRaisesRegex(SystemExit, "PROD_ENV must contain"):
            self.run_prepare()
        self.assertFalse(self.output.exists())

    def test_existing_last_line_gets_separator_without_losing_content(self):
        self.guidance()
        self.env["PROD_ENV"] = "UNKNOWN_SECRET=preserve-without-newline  "
        self.run_prepare()
        self.assertTrue(self.merged().startswith("UNKNOWN_SECRET=preserve-without-newline  \n"))


if __name__ == "__main__":
    unittest.main()
