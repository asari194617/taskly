import os, pathlib, signal, subprocess, tempfile, time, unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]

class RunnerStopTest(unittest.TestCase):
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

if __name__ == '__main__':
    unittest.main()
