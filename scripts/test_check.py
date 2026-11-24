import contextlib
import importlib.util
import io
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import DEFAULT, call, patch

SCRIPT = Path(__file__).with_name('check.py')
spec = importlib.util.spec_from_file_location('check_script', SCRIPT)
check_script = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check_script)


class ReleaseVersionTest(unittest.TestCase):
    def test_repository_references_agree(self):
        check_script.check_release_versions()

    def test_mismatched_files_and_versions_are_reported(self):
        framework = 'github.com/FlyTOmeLight/server-go'
        files = {
            'README.md': (f'go get {framework}@v1.2.3\n并引用发布版框架 v1.2.3。\n'
                          '## v0.6.0 接口调整\n## v1.2.3\n## v1.2.2\n'),
            'examples/README.md': f'`{framework}@v1.2.4`\n',
            'scripts/gen_code.py': f'"\\t{framework} v1.2.3\\n"\n',
            'examples/example-hello-world/go.mod': f'require (\n\t{framework} v1.2.3\n)\n',
            'examples/example-service1/go.mod': f'require (\n\t{framework} v1.2.3\n)\n',
            'examples/example-service2/go.mod': f'require {framework} v1.2.3\n',
            'examples/example-combine-service/go.mod': f'require (\n\t{framework} v1.2.5\n)\n',
        }
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for relative, content in files.items():
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(content, encoding='utf-8')
            with self.assertRaisesRegex(RuntimeError, 'Inconsistent server-go release versions') as raised:
                check_script.check_release_versions(root)
            message = str(raised.exception)
            self.assertIn('examples/README.md (framework reference): v1.2.4', message)
            self.assertIn('examples/example-combine-service/go.mod (framework dependency): v1.2.5', message)
            self.assertIn('README.md (release heading): v1.2.3\n', message)


class FastCheckTest(unittest.TestCase):
    def test_fast_runs_shared_checks_without_docker_or_example_servers(self):
        with patch.multiple(check_script, postgres_database=DEFAULT, mysql_database=DEFAULT,
                            check_examples=DEFAULT, check_generated=DEFAULT, check_release_versions=DEFAULT,
                            check_module=DEFAULT, run=DEFAULT) as steps, \
                patch.object(check_script.shutil, 'which', side_effect={'go': '/fake/go',
                                                                        'gorgeous': '/fake/gorgeous'}.get), \
                patch.object(check_script.signal, 'signal'), contextlib.redirect_stdout(io.StringIO()):
            check_script.main(['--fast'])

        for name in ('postgres_database', 'mysql_database', 'check_examples', 'check_generated'):
            steps[name].assert_not_called()
        steps['check_release_versions'].assert_called_once_with()
        self.assertEqual(steps['check_module'].call_args_list,
                         [call(module) for module in [check_script.ROOT, *check_script.EXAMPLES]])
        root_race, example_race, python_tests = steps['run'].call_args_list
        self.assertEqual(root_race.args[:4], ('go', 'test', '-count=1', '-race'))
        self.assertTrue({'./pkg/cache', './pkg/safety', './version', './swagger'} <= set(root_race.args))
        self.assertEqual(example_race, call('go', 'test', '-count=1', '-race', './service', './sdk/go/...',
                                           cwd=check_script.EXAMPLES[0]))
        self.assertEqual(python_tests, call(sys.executable, '-B', '-m', 'unittest', 'discover',
                                           '-s', 'scripts', '-p', 'test_*.py'))


class CommandFailureTest(unittest.TestCase):
    def test_captured_failure_prints_tool_output_and_keeps_exit_code(self):
        failure = subprocess.CalledProcessError(2, ['gorgeous'],
                                               output='formatter details\n', stderr='formatter error\n')
        diagnostic = io.StringIO()
        with patch.object(check_script.subprocess, 'run', side_effect=failure), \
                contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(diagnostic):
            with self.assertRaises(subprocess.CalledProcessError) as raised:
                check_script.run('gorgeous', capture=True)
        self.assertEqual(raised.exception.returncode, 2)
        self.assertIn('formatter details', diagnostic.getvalue())
        self.assertIn('formatter error', diagnostic.getvalue())


class PostgreSQLFixtureTest(unittest.TestCase):
    def test_missing_docker_fails_without_skipping(self):
        with patch.object(check_script.shutil, 'which', return_value=None):
            with self.assertRaisesRegex(RuntimeError, 'Docker is required'):
                with check_script.postgres_database():
                    self.fail('fixture succeeded without Docker')

    def test_owned_container_is_removed_on_success_and_failures(self):
        for failure in (None, 'create', 'start', 'readiness', 'verification'):
            with self.subTest(failure=failure):
                removals = []

                def fake_docker(*args, **kwargs):
                    action = args[0]
                    if action == 'create':
                        if failure == 'create':
                            raise RuntimeError('create failed')
                        cidfile = Path(args[args.index('--cidfile') + 1])
                        cidfile.write_text('owned-container')
                    if action == 'start' and failure == 'start':
                        raise RuntimeError('start failed')
                    if action == 'rm':
                        removals.append(args[-1])
                    code = 1 if action == 'exec' and failure == 'readiness' else 0
                    output = '127.0.0.1:54321\n' if action == 'port' else ''
                    return subprocess.CompletedProcess(args, code, output, '')

                with patch.object(check_script.shutil, 'which', return_value='/fake/docker'), \
                        patch.object(check_script, 'docker', side_effect=fake_docker), \
                        patch.object(check_script.time, 'monotonic', side_effect=[0, 31]):
                    if failure is None:
                        with check_script.postgres_database() as dsn:
                            self.assertIn('@127.0.0.1:54321/servergo?sslmode=disable', dsn)
                    else:
                        with self.assertRaises(RuntimeError):
                            with check_script.postgres_database():
                                if failure == 'verification':
                                    raise RuntimeError('verification failed')
                                self.fail('failed fixture reached verification')
                self.assertEqual(removals, [] if failure == 'create' else ['owned-container'])

    def test_cleanup_failure_is_reported(self):
        def fake_docker(*args, **kwargs):
            if args[0] == 'create':
                Path(args[args.index('--cidfile') + 1]).write_text('owned-container')
            if args[0] == 'rm':
                return subprocess.CompletedProcess(args, 1, '', 'daemon unavailable')
            output = '127.0.0.1:54321\n' if args[0] == 'port' else ''
            return subprocess.CompletedProcess(args, 0, output, '')

        with patch.object(check_script.shutil, 'which', return_value='/fake/docker'), \
                patch.object(check_script, 'docker', side_effect=fake_docker):
            with self.assertRaisesRegex(RuntimeError, 'Failed to clean up PostgreSQL container'):
                with check_script.postgres_database():
                    pass


class MySQLFixtureTest(unittest.TestCase):
    def test_owned_container_is_removed_on_success_and_failures(self):
        for failure in (None, 'create', 'start', 'readiness', 'verification'):
            with self.subTest(failure=failure):
                removals = []

                def fake_docker(*args, **kwargs):
                    action = args[0]
                    if action == 'create':
                        if failure == 'create':
                            raise RuntimeError('create failed')
                        Path(args[args.index('--cidfile') + 1]).write_text('owned-mysql')
                    if action == 'start' and failure == 'start':
                        raise RuntimeError('start failed')
                    if action == 'rm':
                        removals.append(args[-1])
                    code = 1 if action == 'exec' and failure == 'readiness' else 0
                    output = '127.0.0.1:54321\n' if action == 'port' else ''
                    return subprocess.CompletedProcess(args, code, output, '')

                with patch.object(check_script.shutil, 'which', return_value='/fake/docker'), \
                        patch.object(check_script, 'docker', side_effect=fake_docker), \
                        patch.object(check_script.time, 'monotonic', side_effect=[0, 91]):
                    if failure is None:
                        with check_script.mysql_database() as dsn:
                            self.assertIn('@tcp(127.0.0.1:54321)/servergo', dsn)
                    else:
                        with self.assertRaises(RuntimeError):
                            with check_script.mysql_database():
                                if failure == 'verification':
                                    raise RuntimeError('verification failed')
                                self.fail('failed fixture reached verification')
                self.assertEqual(removals, [] if failure == 'create' else ['owned-mysql'])


if __name__ == '__main__':
    unittest.main()
