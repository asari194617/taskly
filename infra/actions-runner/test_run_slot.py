import os, pathlib, signal, subprocess, tempfile, time, unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]

class RunnerStopTest(unittest.TestCase):
    def test_rejects_unknown_slot(self):
        proc = subprocess.run(
            ['bash', str(ROOT/'infra/actions-runner/run-slot.sh'), 'unknown'],
            capture_output=True,
            text=True,
        )
        self.assertNotEqual(proc.returncode, 0)
        self.assertIn('or prod', proc.stderr)

    def test_term_cleans_workspace(self):
        with tempfile.TemporaryDirectory() as temp:
            d = pathlib.Path(temp)
            (d/'stat').write_text('#!/bin/sh\necho 1001\n')
            (d/'mktemp').write_text('#!/bin/sh\nmkdir "$CASE_DIR/work"\necho "$CASE_DIR/work"\n')
            (d/'docker').write_text('''#!/usr/bin/env python3
import os, pathlib, sys, time
p = pathlib.Path(os.environ['CASE_DIR'])
a = sys.argv[1:]
if '--name' in a:
    (p/'running').touch()
    while not (p/'stopped').exists(): time.sleep(.02)
elif '/workspace' in ' '.join(a):
    (p/'work'/'runner').mkdir()
    (p/'work'/'runner'/'job-file').write_text('fixture')
elif '/cleanup' in ' '.join(a):
    import shutil
    shutil.rmtree(p/'work'/'runner')
    (p/'cleaned').touch()
''')
            for name in ['stat','mktemp','docker']: (d/name).chmod(0o755)
            env = dict(os.environ, PATH=str(d)+':'+os.environ['PATH'], CASE_DIR=str(d))
            proc = subprocess.Popen(['bash',str(ROOT/'infra/actions-runner/run-slot.sh'),'1'],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            try:
                for _ in range(250):
                    if (d/'running').exists(): break
                    time.sleep(.02)
                self.assertTrue((d/'running').exists())
                # systemd TERM while Docker exits after ExecStop must run EXIT cleanup.
                proc.send_signal(signal.SIGTERM)
                (d/'stopped').touch()
                out,err = proc.communicate(timeout=5)
                self.assertEqual(proc.returncode,0,(out,err))
                self.assertTrue((d/'cleaned').exists())
                self.assertFalse((d/'work').exists())
            finally:
                if proc.poll() is None:
                    proc.kill(); proc.wait()

    def test_initialization_failure_cleans_workspace(self):
        with tempfile.TemporaryDirectory() as temp:
            d = pathlib.Path(temp)
            (d/'stat').write_text('#!/bin/sh\necho 1001\n')
            (d/'mktemp').write_text('#!/bin/sh\nmkdir "$CASE_DIR/work"\necho "$CASE_DIR/work"\n')
            (d/'docker').write_text('''#!/usr/bin/env python3
import os, pathlib, shutil, sys
p = pathlib.Path(os.environ['CASE_DIR'])
a = sys.argv[1:]
if '/workspace' in ' '.join(a):
    (p/'work'/'partial').write_text('fixture')
    raise SystemExit(1)
if '/cleanup' in ' '.join(a):
    for child in (p/'work').iterdir():
        if child.is_dir(): shutil.rmtree(child)
        else: child.unlink()
''')
            for name in ['stat','mktemp','docker']: (d/name).chmod(0o755)
            env = dict(os.environ, PATH=str(d)+':'+os.environ['PATH'], CASE_DIR=str(d))
            proc = subprocess.run(
                ['bash',str(ROOT/'infra/actions-runner/run-slot.sh'),'1'],
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertNotEqual(proc.returncode, 0)
            self.assertFalse((d/'work').exists())

    def test_prod_uses_dedicated_container_and_config(self):
        with tempfile.TemporaryDirectory() as temp:
            d = pathlib.Path(temp)
            (d/'stat').write_text('#!/bin/sh\necho 1001\n')
            (d/'mktemp').write_text('#!/bin/sh\nmkdir "$CASE_DIR/work"\necho "$CASE_DIR/work"\n')
            (d/'docker').write_text('''#!/usr/bin/env python3
import os, pathlib, sys
p = pathlib.Path(os.environ['CASE_DIR'])
a = sys.argv[1:]
if '--name' in a:
    (p/'args').write_text('\\n'.join(a))
elif '/workspace' in ' '.join(a):
    (p/'work'/'runner').mkdir()
elif '/cleanup' in ' '.join(a):
    import shutil
    shutil.rmtree(p/'work'/'runner')
''')
            for name in ['stat','mktemp','docker']: (d/name).chmod(0o755)
            env = dict(os.environ, PATH=str(d)+':'+os.environ['PATH'], CASE_DIR=str(d))
            proc = subprocess.run(
                ['bash',str(ROOT/'infra/actions-runner/run-slot.sh'),'prod'],
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(proc.returncode, 0, (proc.stdout, proc.stderr))
            args = (d/'args').read_text()
            self.assertIn('taskly-prod', args)
            self.assertIn('source=taskly-runner-config-prod', args)

    def test_registration_uses_host_specific_name_and_label(self):
        with tempfile.TemporaryDirectory() as temp:
            d = pathlib.Path(temp)
            (d/'runner.env').write_text(
                'TASKLY_RUNNER_IMAGE=test-runner\n'
                'TASKLY_RUNNER_NAME_PREFIX=taskly-ci-primary-slot\n'
                'TASKLY_RUNNER_CI_LABEL=taskly-ci-primary\n'
            )
            (d/'docker').write_text(
                '#!/bin/sh\nprintf "%s\\n" "$@" >> "$CASE_DIR/docker-args"\n'
            )
            (d/'docker').chmod(0o755)
            env = dict(
                os.environ,
                PATH=str(d)+':'+os.environ['PATH'],
                CASE_DIR=str(d),
                TASKLY_RUNNER_ENV_FILE=str(d/'runner.env'),
            )
            proc = subprocess.run(
                ['bash', str(ROOT/'infra/actions-runner/register-slot.sh'), '2'],
                env=env,
                input='registration-token',
                capture_output=True,
                text=True,
            )
            self.assertEqual(proc.returncode, 0, (proc.stdout, proc.stderr))
            args = (d/'docker-args').read_text()
            self.assertIn('RUNNER_NAME=taskly-ci-primary-slot-2', args)
            self.assertIn('RUNNER_LABELS=taskly-ci-primary', args)
            self.assertIn('test-runner', args)

if __name__ == '__main__':
    unittest.main()
