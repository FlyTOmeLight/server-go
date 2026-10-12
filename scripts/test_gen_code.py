import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name('gen_code.py')
spec = importlib.util.spec_from_file_location('gen_code', SCRIPT)
gen_code = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gen_code)
TEMPLATE = SCRIPT.parent.parent / 'examples' / 'example-hello-world'


class GeneratorTest(unittest.TestCase):
    def test_existing_target_is_preserved_and_cli_fails(self):
        with tempfile.TemporaryDirectory() as output:
            target = Path(output) / 'test-project'
            target.mkdir()
            existing = target / 'existing-work.txt'
            existing.write_text('user code')
            legacy_temp = Path(output) / 'test-project_temp'
            legacy_temp.mkdir()
            result = subprocess.run([
                sys.executable, str(SCRIPT), '--template', str(TEMPLATE),
                '--output', output, '--name', 'TestProject',
                '--go-mod', 'example.com/test-project',
            ], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(existing.read_text(), 'user code')
            self.assertTrue(legacy_temp.is_dir())
            self.assertEqual(set(Path(output).iterdir()), {target, legacy_temp})

    def test_processing_failure_propagates_and_cleans_staging(self):
        with tempfile.TemporaryDirectory() as output:
            generator = gen_code.MultiModuleGenerator(str(TEMPLATE))
            with patch.object(generator, '_process_project', side_effect=OSError('cannot write')):
                with self.assertRaisesRegex(OSError, 'cannot write'):
                    generator.generate('TestProject', output, ['Module'], 'example.com/test-project')
            self.assertEqual(list(Path(output).iterdir()), [])

    def test_target_created_during_generation_is_preserved(self):
        with tempfile.TemporaryDirectory() as output:
            generator = gen_code.MultiModuleGenerator(str(TEMPLATE))
            target = Path(output) / 'test-project'
            def concurrent_project(*args):
                target.mkdir()
                (target / 'user.txt').write_text('another invocation')
            with patch.object(generator, '_process_project', side_effect=concurrent_project):
                with self.assertRaises(FileExistsError):
                    generator.generate('TestProject', output, ['Module'], 'example.com/test-project')
            self.assertEqual((target / 'user.txt').read_text(), 'another invocation')
            self.assertEqual(list(Path(output).iterdir()), [target])

    def test_default_and_custom_modules(self):
        for modules in [['Module'], ['Catalog'], ['Catalog', 'Order']]:
            with self.subTest(modules=modules), tempfile.TemporaryDirectory() as output:
                generator = gen_code.MultiModuleGenerator(str(TEMPLATE))
                with contextlib.redirect_stdout(io.StringIO()):
                    target = Path(generator.generate('TestProject', output, modules, 'example.com/test-project'))
                self.assertIn('module example.com/test-project', (target / 'go.mod').read_text())
                self.assertEqual(list(Path(output).iterdir()), [target])
                self.assertFalse(list(target.rglob('*.tmp*')))
                for module in modules:
                    self.assertTrue((target / 'internal' / 'app' / module.lower()).is_dir())
                self.assertNotIn('github.com/FlyTOmeLight/server-go/examples/example-hello-world',
                                 '\n'.join(path.read_text() for path in target.rglob('*.go')))
                self.assertTrue((target / 'internal' / 'service' / 'service.go').is_file())
                self.assertFalse((target / 'service').exists())
                for lifecycle_test in (target / 'internal' / 'service').glob('*lifecycle_test.go'):
                    self.assertIn('&db.Module{', lifecycle_test.read_text())
                spec_file = next((target / 'doc' / 'openapi').glob('*_swagger.json'))
                spec = json.loads(spec_file.read_text())
                for module in modules:
                    self.assertIn('/v1/Create' + module, spec['paths'])
                    self.assertIn('testprojectv1.' + module, spec['definitions'])
                if 'Module' not in modules:
                    self.assertNotIn('/v1/CreateModule', spec['paths'])



if __name__ == '__main__':
    unittest.main()
