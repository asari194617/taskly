import contextlib
import importlib.util
import io
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


if __name__ == "__main__":
    unittest.main()
