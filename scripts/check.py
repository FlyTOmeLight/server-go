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
import uuid

ROOT = Path(__file__).resolve().parent.parent
EXAMPLES = [ROOT / 'examples' / name for name in (
    'example-hello-world', 'example-service1', 'example-service2', 'example-combine-service',
)]
ENV = dict(os.environ, CGO_ENABLED='1', GOWORK='off')


def docker(*args, check=True, timeout=30):
    result = subprocess.run(['docker', *args], env=ENV, text=True,
                            capture_output=True, timeout=timeout)
    if check and result.returncode:
        raise RuntimeError(f'Docker {args[0]} failed: {result.stderr.strip() or result.stdout.strip()}')
    return result


@contextlib.contextmanager
def postgres_database():
    """Own one temporary PostgreSQL container for the full verification run."""
    if shutil.which('docker') is None:
        raise RuntimeError('Docker is required for PostgreSQL checks; install Docker and start its daemon')
    docker('info')
    with tempfile.TemporaryDirectory(prefix='server-go-postgres-') as temporary:
        cidfile = Path(temporary) / 'container.id'
        name = 'server-go-check-' + uuid.uuid4().hex
        password = uuid.uuid4().hex
        try:
            docker('create', '--cidfile', str(cidfile), '--name', name, '--rm',
                   '--tmpfs', '/var/lib/postgresql/data', '--publish', '127.0.0.1::5432',
                   '--env', 'POSTGRES_USER=servergo', '--env', f'POSTGRES_PASSWORD={password}',
                   '--env', 'POSTGRES_DB=servergo', 'postgres:17-alpine', timeout=180)
            container = cidfile.read_text().strip()
            docker('start', container)
            deadline = time.monotonic() + 30
            while True:
                ready = docker('exec', container, 'pg_isready', '-h', '127.0.0.1',
                               '-U', 'servergo', '-d', 'servergo', '-t', '1', check=False, timeout=5)
                if ready.returncode == 0:
                    break
                if time.monotonic() >= deadline:
                    logs = docker('logs', container, check=False).stdout
                    raise RuntimeError(f'PostgreSQL did not become ready within 30 seconds:\n{logs}')
                time.sleep(0.1)
            address = docker('port', container, '5432/tcp').stdout.strip()
            host, port = address.rsplit(':', 1)
            if host != '127.0.0.1' or not port.isdecimal():
                raise RuntimeError(f'Unexpected PostgreSQL port mapping: {address}')
            yield f'postgres://servergo:{password}@127.0.0.1:{port}/servergo?sslmode=disable'
        finally:
            if cidfile.exists():
                container = cidfile.read_text().strip()
                removed = docker('rm', '--force', '--volumes', container, check=False)
                if removed.returncode and not ('No such container' in removed.stderr):
                    raise RuntimeError(f'Failed to clean up PostgreSQL container {container}: {removed.stderr.strip()}')


def run(*args, cwd=ROOT, capture=False):
    print(f'[{cwd.name}] {" ".join(map(str, args))}', flush=True)
    try:
        result = subprocess.run(list(map(str, args)), cwd=cwd, env=ENV,
                                text=True, capture_output=capture, check=True)
    except subprocess.CalledProcessError as error:
        if capture:
            for output in (error.stdout, error.stderr):
                if output:
                    print(output, file=sys.stderr, end='')
        raise
    return result.stdout if capture else None


def check_module(directory):
    # Compile through the required tests before gorgeous's bounded package load.
    run('go', 'test', '-count=1', './...', cwd=directory)
    diff = run('gorgeous', '-d', './...', cwd=directory, capture=True)
    if diff.strip():
        raise RuntimeError(f'Formatting differs in {directory}:\n{diff}')
    run('go', 'vet', './...', cwd=directory)


@contextlib.contextmanager
def running_server(binary, directory, app_name, database=False, postgres=False):
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    config = directory / 'config.toml'
    content = (f'AppName = "{app_name}"\nAppCode = 100\nRunMode = "release"\n'
               f'[HTTPServer]\nListen = "127.0.0.1:{port}"\nAuthType = "Fake"\n')
    if database:
        content += f'[DB]\nDriver = "SQLite"\nDB = "{directory / "smoke.db"}"\nAutoMigrate = true\n'
    if postgres:
        content += '[DB]\nDriver = "PostgreSQL"\nDSN = "${env:DATABASE_URL}"\nAutoMigrate = true\n'
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


def request_json(endpoint, path, body=None, method=None, status=200):
    data = json.dumps(body).encode() if body is not None else None
    request = urllib.request.Request(endpoint + path, data=data, method=method,
                                     headers={'Content-Type': 'application/json'})
    try:
        response = urllib.request.urlopen(request, timeout=3)
    except urllib.error.HTTPError as error:
        if error.code != status:
            raise
        response = error
    with response:
        if response.status != status:
            raise RuntimeError(f'{request.get_method()} {path}: status={response.status}, expected={status}')
        return json.load(response) if status != 204 else None


def check_examples(directory):
    minimal = directory / 'minimal'
    minimal.mkdir()
    binary = minimal / 'bin/minimal-server'
    run('make', 'package', f'OUTDIR={minimal}', cwd=ROOT / 'examples/minimal-service')
    with running_server(binary, minimal, 'minimal-service') as endpoint:
        if request_json(endpoint, '/hello') != {'message': 'hello'}:
            raise RuntimeError('Minimal service /hello contract failed')
    streaming = directory / 'streaming'
    streaming.mkdir()
    binary = streaming / 'bin/streaming-server'
    run('make', 'package', f'OUTDIR={streaming}', cwd=ROOT / 'examples/streaming-service')
    with running_server(binary, streaming, 'streaming-service') as endpoint:
        with urllib.request.urlopen(endpoint + '/stream?name=reader', timeout=5) as response:
            body = response.read().decode()
        if body.count('event:message') != 3 or not body.endswith('event:done\ndata:{}\n\n'):
            raise RuntimeError(f'Streaming service contract failed: {body}')

    for module, server_name, cli_name in (
        (EXAMPLES[0], 'helloworld-server', 'helloworld'),
        (EXAMPLES[1], 'example1-server', 'example1'),
        (EXAMPLES[2], 'example2-server', None),
        (EXAMPLES[3], 'combine-server', 'combine'),
    ):
        package = directory / module.name
        cli_config = package / 'cli.yaml'
        run('make', 'package', f'OUTDIR={package}',
            f'LD_FLAGS=-X github.com/FlyTOmeLight/server-go/cli.ConfigFilePath={cli_config}', cwd=module)
        if not (package / 'conf/server/config.toml').is_file():
            raise RuntimeError(f'Packaged configuration missing: {module.name}')
        with running_server(package / 'bin' / server_name, package, module.name,
                            database=module != EXAMPLES[2]) as endpoint:
            if module in (EXAMPLES[2], EXAMPLES[3]):
                if request_json(endpoint, '/service2/hello') != {'message': 'hello from service2'}:
                    raise RuntimeError('Stateless service contract failed')
            if cli_name:
                cli_config.write_text(f'core:\n  endpoint: {endpoint}\nlog:\n  handler: NONE\n')
                binary = package / 'bin' / cli_name
                if module in (EXAMPLES[0], EXAMPLES[3]):
                    run(binary, 'module', 'create', '--project-id=project1', '--name=cli-example', cwd=package)
                    run(binary, 'module', 'list', cwd=package)
                    spec = package / 'apply.yaml'
                    spec.write_text('kind: Module\nmetadata:\n  projectID: project1\n  name: apply-example\n')
                    run(binary, 'apply', '-f', spec, cwd=package)
                    listed = request_json(endpoint, '/helloworld/internal/v1/ListModule',
                                          {'pageNo': 1, 'pageSize': 10})
                    if listed.get('totalCount') != 2:
                        raise RuntimeError('Module CLI create/apply contract failed')
                if module in (EXAMPLES[1], EXAMPLES[3]):
                    run(binary, 'dataset', 'create', '--project-name=project1',
                        '--local-name=cli-example', '--file-format=File', cwd=package)
                    dataset = request_json(endpoint, '/v1/projects/project1/datasets/cli-example')
                    if dataset.get('fileFormat') != 'File':
                        raise RuntimeError('Dataset CLI request lost fileFormat')
                    run(binary, 'dataset', 'list', '--project-name=project1', cwd=package)
                    spec = package / 'dataset.yaml'
                    spec.write_text('kind: Dataset\nmetadata:\n  projectName: project1\n  localName: apply-example\n')
                    run(binary, 'apply', '-f', spec, cwd=package)
                    listed = request_json(endpoint, '/v1/projects/project1/datasets?filter=apply-example')
                    if listed.get('totalCount') != 1:
                        raise RuntimeError('Dataset filter/apply contract failed')
    worker_package = directory / 'worker'
    worker = worker_package / 'bin/leader-worker'
    run('make', 'package', f'OUTDIR={worker_package}', cwd=ROOT / 'examples/example-leader-election')
    result = run(worker, '--local', '--id=local', '--time=1200ms', cwd=directory, capture=True)
    if 'task is cancelled' not in result:
        raise RuntimeError('Leader worker did not join its canceled task')
    check_postgresql(directory)


def check_postgresql(directory):
    package = directory / 'postgresql'
    run('make', 'package', f'OUTDIR={package}', cwd=ROOT / 'examples/postgresql-service')
    if not (package / 'conf/server/config.toml').is_file():
        raise RuntimeError('Packaged PostgreSQL configuration missing')
    with running_server(package / 'bin/postgresql-server', package, 'postgresql-service', postgres=True) as endpoint:
        default = request_json(endpoint, '/v1/notes', {'name': 'defaults'}, status=201)
        if default.get('priority') != 2 or default.get('enabled') is not True or default.get('reviewed') is not None:
            raise RuntimeError(f'PostgreSQL defaults were lost: {default}')
        body = {'name': 'editable', 'description': 'created', 'priority': 0, 'enabled': False, 'reviewed': False}
        created = request_json(endpoint, '/v1/notes', body, status=201)
        if any(created.get(key) != value for key, value in body.items()) or not created.get('id'):
            raise RuntimeError(f'PostgreSQL create lost explicit values: {created}')
        fetched = request_json(endpoint, '/v1/notes/' + created['id'])
        if fetched != created:
            raise RuntimeError('PostgreSQL read changed the created note')
        listed = request_json(endpoint, '/v1/notes?pageNo=2&pageSize=1')
        if listed.get('totalCount') != 2 or listed.get('pageNo') != 2 or listed.get('pageSize') != 1 or len(listed.get('results', [])) != 1:
            raise RuntimeError(f'PostgreSQL pagination contract failed: {listed}')
        default_page = request_json(endpoint, '/v1/notes')
        if default_page.get('pageNo') != 1 or default_page.get('pageSize') != 20 or default_page.get('totalCount') != 2:
            raise RuntimeError(f'PostgreSQL pagination defaults failed: {default_page}')
        body.update(description='updated', priority=4, enabled=True, reviewed=True)
        updated = request_json(endpoint, '/v1/notes/' + created['id'], body, method='PUT')
        if any(updated.get(key) != value for key, value in body.items()):
            raise RuntimeError(f'PostgreSQL update failed: {updated}')
        if request_json(endpoint, '/v1/notes/' + created['id']) != updated:
            raise RuntimeError('PostgreSQL update was not persisted')
        for path, invalid, method in (
            ('/v1/notes', {'name': ''}, 'POST'),
            ('/v1/notes', {'name': 'invalid', 'priority': 11}, 'POST'),
            ('/v1/notes?pageNo=0', None, 'GET'),
            ('/v1/notes?pageSize=101', None, 'GET'),
            ('/v1/notes/not-a-uuid', None, 'GET'),
        ):
            request_json(endpoint, path, invalid, method=method, status=400)
        for note in (default, created):
            request_json(endpoint, '/v1/notes/' + note['id'], method='DELETE', status=204)
            request_json(endpoint, '/v1/notes/' + note['id'], status=404)
        if request_json(endpoint, '/v1/notes').get('totalCount') != 0:
            raise RuntimeError('PostgreSQL delete did not remove both notes')


def check_generated(directory, name, modules):
    run(sys.executable, ROOT / 'scripts/gen_code.py', '--template', ROOT / 'examples/example-hello-world',
        '--output', directory, '--name', name, '--modules', *modules, '--go-mod', f'example.com/{name.lower()}')
    project = directory / name.lower()
    # The replacement exists only in the temporary project, to verify unpublished changes.
    run('go', 'mod', 'edit', f'-replace=github.com/FlyTOmeLight/server-go={ROOT}', cwd=project)
    run('go', 'mod', 'tidy', cwd=project)
    run('gorgeous', './...', cwd=project, capture=True)
    run('make', 'check', cwd=project)
    run('make', 'package', cwd=project)
    binary = project / 'output/bin' / f'{name.lower()}-server'
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
    # Turn termination into an exception so the contexts still remove owned resources.
    signal.signal(signal.SIGTERM, lambda signum, frame: sys.exit(128 + signum))
    with postgres_database() as dsn:
        ENV.update(SERVER_GO_TEST_POSTGRES_DSN=dsn, DATABASE_URL=dsn)
        for module in [ROOT, *EXAMPLES]:
            check_module(module)
        run('go', 'test', '-count=1', '-race', './app', '.', './host', './middleware', './console', './errors', './pkg',
            './stream', './cron', './pkg/binder', './pkg/election', './examples/streaming-service/...', './examples/postgresql-service/...')
        run('go', 'test', '-count=1', '-race', './service', './sdk/go/...', cwd=EXAMPLES[0])
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
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired, RuntimeError) as error:
        print(f'Check failed: {error}', file=sys.stderr)
        sys.exit(1)
