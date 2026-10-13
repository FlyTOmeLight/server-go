"""Check the framework, independent examples and generated projects."""
import contextlib
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
EXAMPLES = [ROOT / 'examples' / name for name in (
    'example-hello-world', 'example-service1', 'example-service2', 'example-combine-service',
)]
ENV = dict(os.environ, CGO_ENABLED='1', GOWORK='off')


def run(*args, cwd=ROOT, capture=False):
    print(f'[{cwd.name}] {" ".join(map(str, args))}', flush=True)
    result = subprocess.run(list(map(str, args)), cwd=cwd, env=ENV,
                            text=True, capture_output=capture, check=True)
    return result.stdout if capture else None


def check_module(directory):
    diff = run('gorgeous', '-d', './...', cwd=directory, capture=True)
    if diff.strip():
        raise RuntimeError(f'Formatting differs in {directory}:\n{diff}')
    run('go', 'test', './...', cwd=directory)
    run('go', 'vet', './...', cwd=directory)


@contextlib.contextmanager
def running_server(binary, directory, app_name, database=False):
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    config = directory / 'config.toml'
    content = (f'AppName = "{app_name}"\nAppCode = 100\nRunMode = "release"\n'
               f'[HTTPServer]\nListen = "127.0.0.1:{port}"\nAuthType = "Fake"\n')
    if database:
        content += f'[DB]\nDriver = "SQLite"\nDB = "{directory / "smoke.db"}"\nAutoMigrate = true\n'
    config.write_text(content)
    with tempfile.TemporaryFile(mode='w+t') as log:
        process = subprocess.Popen([str(binary)], cwd=directory,
                                   env=dict(ENV, SERVER_CONFIG_PATH=str(config)),
                                   stdout=log, stderr=log)
        endpoint = f'http://127.0.0.1:{port}'
        try:
            deadline = time.monotonic() + 10
            while True:
                if process.poll() is not None:
                    log.seek(0)
                    raise RuntimeError(f'Server exited before readiness:\n{log.read()}')
                try:
                    with urllib.request.urlopen(endpoint + '/healthz', timeout=1) as response:
                        if response.status == 200:
                            break
                except (urllib.error.URLError, TimeoutError):
                    pass
                if time.monotonic() >= deadline:
                    raise RuntimeError('Server did not become ready within 10 seconds')
                time.sleep(0.05)
            yield endpoint
        finally:
            if process.poll() is None:
                process.send_signal(signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
                    raise RuntimeError('Server did not stop after SIGTERM')
            if process.returncode != 0:
                log.seek(0)
                raise RuntimeError(f'Server exited with {process.returncode}:\n{log.read()}')


def request_json(endpoint, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    request = urllib.request.Request(endpoint + path, data=data,
                                     headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(request, timeout=3) as response:
        return json.load(response)


def check_examples(directory):
    minimal = directory / 'minimal'
    minimal.mkdir()
    binary = minimal / 'server'
    run('go', 'build', '-o', binary, './examples/minimal-service')
    with running_server(binary, minimal, 'minimal-service') as endpoint:
        if request_json(endpoint, '/hello') != {'message': 'hello'}:
            raise RuntimeError('Minimal service /hello contract failed')
    streaming = directory / 'streaming'
    streaming.mkdir()
    binary = streaming / 'server'
    run('go', 'build', '-o', binary, './examples/streaming-service/cmd/server')
    with running_server(binary, streaming, 'streaming-service') as endpoint:
        with urllib.request.urlopen(endpoint + '/stream?name=reader', timeout=5) as response:
            body = response.read().decode()
        if body.count('event:message') != 3 or not body.endswith('event:done\ndata:{}\n\n'):
            raise RuntimeError(f'Streaming service contract failed: {body}')


def check_generated(directory, name, modules):
    run(sys.executable, ROOT / 'scripts/gen_code.py', '--template', ROOT / 'examples/example-hello-world',
        '--output', directory, '--name', name, '--modules', *modules, '--go-mod', f'example.com/{name.lower()}')
    project = directory / name.lower()
    # The replacement exists only in the temporary project, to verify unpublished changes.
    run('go', 'mod', 'edit', f'-replace=github.com/FlyTOmeLight/server-go={ROOT}', cwd=project)
    run('go', 'mod', 'tidy', cwd=project)
    run('gorgeous', './...', cwd=project, capture=True)
    run('make', 'check', cwd=project)
    binary = project / 'server'
    run('go', 'build', '-o', binary, './cmd/server', cwd=project)
    run('go', 'build', '-o', project / 'cli', './cmd/cli', cwd=project)
    with running_server(binary, project, name.lower(), database=True) as endpoint:
        for module in modules:
            prefix = f'/{name.lower()}/internal/v1/'
            body = {'name': 'smoke', 'projectID': 'project1'}
            created = request_json(endpoint, prefix + 'Create' + module + '?clientToken=smoke', body)
            replayed = request_json(endpoint, prefix + 'Create' + module + '?clientToken=smoke', body)
            if created != replayed or not created.get('id'):
                raise RuntimeError('Generated create/idempotence contract failed')
            listed = request_json(endpoint, prefix + 'List' + module, {'pageNo': 1, 'pageSize': 10})
            if listed.get('totalCount') != 1:
                raise RuntimeError('Generated list contract failed')


def main():
    for command in ('go', 'gorgeous', 'make'):
        if shutil.which(command) is None:
            raise RuntimeError(f'Missing required command: {command}')
    for module in [ROOT, *EXAMPLES]:
        check_module(module)
    run('go', 'test', '-race', './host', './middleware', './console', './errors', './pkg', './stream',
        './examples/streaming-service/...')
    run('go', 'test', '-race', './internal/service', './sdk/go/...', cwd=EXAMPLES[0])
    run(sys.executable, '-B', '-m', 'unittest', 'discover', '-s', 'scripts', '-p', 'test_*.py')
    with tempfile.TemporaryDirectory(prefix='server-go-check-') as temporary:
        directory = Path(temporary)
        check_examples(directory)
        check_generated(directory, 'Generated', ['Module'])
        check_generated(directory, 'Multi', ['Catalog', 'Order'])
    print('All checks passed.', flush=True)


if __name__ == '__main__':
    try:
        main()
    except (subprocess.CalledProcessError, RuntimeError) as error:
        print(f'Check failed: {error}', file=sys.stderr)
        sys.exit(1)
