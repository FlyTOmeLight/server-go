import contextlib
import importlib.util
import io
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name('check.py')
spec = importlib.util.spec_from_file_location('check_script', SCRIPT)
check_script = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check_script)


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
