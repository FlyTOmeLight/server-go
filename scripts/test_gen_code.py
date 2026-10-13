import contextlib
import importlib.util
import io
import json
import shutil
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
        for modules in [['Module'], ['Catalog'], ['Catalog', 'Order'], ['CatalogItem', 'OrderLine']]:
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
                self.assertTrue((target / 'service' / 'service.go').is_file())
                self.assertFalse((target / 'internal' / 'service').exists())
                for lifecycle_test in (target / 'service').glob('*lifecycle_test.go'):
                    self.assertIn('&db.Module{', lifecycle_test.read_text())
                spec_file = next((target / 'doc' / 'openapi').glob('*_swagger.json'))
                spec = json.loads(spec_file.read_text())
                for module in modules:
                    self.assertIn('/v1/Create' + module, spec['paths'])
                    self.assertIn('testprojectv1.' + module, spec['definitions'])
                if 'Module' not in modules:
                    self.assertNotIn('/v1/CreateModule', spec['paths'])
                build = (target / 'service' / 'build.go').read_text()
                self.assertEqual(build.count('&db.Module{'), 1)
                self.assertNotIn('CatalogController', build)
                for module in modules:
                    filename = gen_code._camel_to_snake(module)
                    for relative in [
                        f'internal/table/{filename}.go', f'command/{filename}.go',
                        f'service/{filename}_lifecycle_test.go', f'service/{filename}_sdk_test.go',
                        f'sdk/go/testprojectv1/testproject_api_{filename}.go',
                        f'sdk/go/testprojectv1/testproject_client_{filename}.go',
                        f'sdk/go/testprojectv1/testproject_client_{filename}_test.go',
                    ]:
                        self.assertTrue((target / relative).is_file(), relative)
                    lifecycle = (target / f'service/{filename}_lifecycle_test.go').read_text()
                    self.assertIn('&db.Module{', lifecycle)
                    if module != 'Module':
                        self.assertNotIn('&db.' + module + '{', lifecycle)
                    self.assertIn(f'{module.lower()}.New{module}Controller', (target / 'service/service.go').read_text())
                    self.assertIn(f'Create{module}(ctx', (target / 'sdk/go/testprojectv1/testproject_api.go').read_text())
                    self.assertIn(f'command.{module}CommandBuilder', (target / 'cmd/cli/cli.go').read_text())
                    for operation in ['Create', 'List', 'Get', 'Update', 'Delete']:
                        self.assertIn('/v1/' + operation + module, spec['paths'])
                yaml = spec_file.with_suffix('.yaml')
                if modules != ['Module']:
                    self.assertEqual(json.loads(yaml.read_text()), spec)
                    docs = spec_file.with_name(spec_file.name.replace('_swagger.json', '_docs.go')).read_text()
                    start = docs.index('"paths":')
                    end = docs.index('}`', start)
                    embedded = json.loads('{' + docs[start:end + 1])
                    self.assertEqual(embedded['paths'], spec['paths'])
                    self.assertEqual(embedded['definitions'], spec['definitions'])

    def test_project_and_domain_names_do_not_replace_each_other(self):
        for name, modules in [('ModuleApp', ['Catalog', 'Order']),
                              ('TestModule', ['HelloWorld', 'ModuleItem']),
                              ('Module', ['module', 'ModuleA'])]:
            with self.subTest(name=name, modules=modules), tempfile.TemporaryDirectory() as output:
                generator = gen_code.MultiModuleGenerator(str(TEMPLATE))
                with contextlib.redirect_stdout(io.StringIO()):
                    target = Path(generator.generate(name, output, modules, 'example.com/' + name.lower()))
                project = name.lower()
                self.assertIn(f'New{name}Service', (target / 'service/service.go').read_text())
                for module in modules:
                    filename = gen_code._camel_to_snake(module)
                    self.assertTrue((target / f'sdk/go/{project}v1/{project}_client_{filename}.go').is_file())
                    controller = (target / f'internal/app/{module.lower()}/controller.go').read_text()
                    self.assertIn(f'/v1/Create{module}', controller)
                    self.assertIn(f'"example.com/{project}/internal/table"', (target / f'internal/app/{module.lower()}/service.go').read_text())
                    self.assertIn(f'/{project}/internal/v1/Create{module}', (target / 'deploy/scripts/curl.sh').read_text())
                    self.assertIn(f'sdk/go/{project}v1/{project}_api_{filename}.go', (target / 'README.md').read_text())
                self.assertFalse(list(target.rglob('*.template')))
                self.assertNotIn('ServerGoProjectToken', '\n'.join(path.read_text() for path in target.rglob('*.go')))

    def test_unlisted_files_receive_project_renaming_only(self):
        with tempfile.TemporaryDirectory() as output, tempfile.TemporaryDirectory() as source:
            template = Path(source) / 'template'
            shutil.copytree(TEMPLATE, template)
            shared = template / 'service' / 'shared.go'
            shared.write_text('package service\n// HelloWorld keeps Module and module unchanged.\n')
            generator = gen_code.MultiModuleGenerator(str(template))
            with contextlib.redirect_stdout(io.StringIO()):
                target = Path(generator.generate('TestProject', output, ['Catalog', 'Order'], 'example.com/test-project'))
            self.assertEqual((target / 'service/shared.go').read_text(),
                             'package service\n// TestProject keeps Module and module unchanged.\n')

    def test_changed_aggregate_template_fails_without_publishing(self):
        with tempfile.TemporaryDirectory() as output, tempfile.TemporaryDirectory() as source:
            template = Path(source) / 'template'
            shutil.copytree(TEMPLATE, template)
            service = template / 'service/service.go'
            service.write_text(service.read_text().replace('moduleController *module.Controller', 'controller *module.Controller'))
            generator = gen_code.MultiModuleGenerator(str(template))
            with contextlib.redirect_stdout(io.StringIO()), self.assertRaisesRegex(ValueError, '模板片段'):
                generator.generate('TestProject', output, ['Catalog'], 'example.com/test-project')
            self.assertEqual(list(Path(output).iterdir()), [])



if __name__ == '__main__':
    unittest.main()
