#!/usr/bin/env python3
"""Synthetic-only Docker checks. Uses disposable named volumes, never Compose."""
import argparse
import base64
import io
import json
import pathlib
import subprocess
import tarfile
import time
import urllib.request
import uuid

p = argparse.ArgumentParser()
p.add_argument('--image', required=True)
p.add_argument('--platform', required=True)
p.add_argument('--build-id', required=True)
a = p.parse_args()
arch = a.platform.split('/')[-1]
probe = pathlib.Path(f'bin/probes/{arch}/operations-probe').resolve()
name = 'energything-ops-' + uuid.uuid4().hex
volumes = [name + suffix for suffix in ('-data', '-backup', '-restore', '-denied')]
sentinel = ('PRIVATE' + '-TASK07-' + uuid.uuid4().hex).encode()
retained_responses = {}


def docker(*args, check=True, input=None):
    return subprocess.run(['docker', *map(str, args)], check=check, input=input,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=300)


def run(*args, **kw):
    return docker('run', '--rm', '--platform', a.platform, *args, **kw)


def helper(volume, mode):
    return run('--network', 'none', '-v', volume + ':/data', '-v', str(probe) + ':/probe:ro',
               '--entrypoint', '/probe', a.image, mode, '/data').stdout


def start(volume):
    docker('run', '-d', '--platform', a.platform, '--name', name,
           '-p', '127.0.0.1::8080', '-v', volume + ':/data',
           '-e', 'ENERGYTHING_USERNAME=synthetic-user',
           '-e', 'ENERGYTHING_PASSWORD=synthetic-password',
           '-e', 'ENERGYTHING_BACKUP_INTERVAL=1s',
           '-e', 'ENERGYTHING_BACKUP_TIMEOUT=20s',
           '-e', 'OCTOPUS_API_KEY=' + sentinel.decode(), a.image)
    base = 'http://' + docker('port', name, '8080/tcp').stdout.decode().strip()
    for _ in range(100):
        try:
            with urllib.request.urlopen(base + '/healthz', timeout=2) as response:
                assert response.read() == b'ok\n'
            break
        except (OSError, TimeoutError):
            time.sleep(.2)
    else:
        raise AssertionError('server not ready')
    assert docker('exec', name, 'id', '-u').stdout.strip() == b'10001'
    docker('exec', name, 'test', '-f', '/var/lib/energything-runtime/energything.sqlite')
    runtime_dirs = docker('exec', name, 'find', '/var/lib/energything-runtime', '-mindepth', '1', '-type', 'd').stdout
    assert not runtime_dirs, 'runtime unexpectedly uses generated subdirectories'
    for binary in ('energything', 'energything-enqueue'):
        copied = docker('cp', name + ':/usr/local/bin/' + binary, '-').stdout
        with tarfile.open(fileobj=io.BytesIO(copied)) as archive:
            elf = archive.extractfile(archive.getmembers()[0]).read(20)
        assert elf[:4] == b'\x7fELF' and int.from_bytes(elf[18:20], 'little') == {'amd64': 62, 'arm64': 183}[arch]
    headers = {'Authorization': 'Basic ' + base64.b64encode(b'synthetic-user:synthetic-password').decode()}
    def check_retained(path, body):
        assert sentinel not in body and b'synthetic-password' not in body
        if path in retained_responses:
            assert retained_responses[path] == body, 'retained API response changed: ' + path
        retained_responses[path] = body

    for path in ('/', '/api/v1/status', '/api/v1/audit',
                 '/api/v1/octopus/intervals?from=2026-10-25&to=2026-10-25',
                 '/api/v1/growatt/samples?view=provider&from=2026-10-01&to=2026-10-31',
                 '/api/v1/growatt/aggregates?from=2026-01-01&to=2026-01-31'):
        with urllib.request.urlopen(urllib.request.Request(base + path, headers=headers), timeout=20) as response:
            body = response.read()
            assert sentinel not in body and b'synthetic-password' not in body
            if path == '/':
                assert b'credentials are incomplete' in body and b'Unconfigured' in body
            if path == '/api/v1/status':
                assert json.loads(body)['build_id'] == a.build_id
            elif path.startswith('/api/'):
                check_retained(path, body)
                payload = json.loads(body)
                assert payload['records'], 'synthetic records missing: ' + path
                if path == '/api/v1/audit':
                    links = [e['url'] for record in payload['records'] for e in record['evidence']]
                    assert links
                    with urllib.request.urlopen(urllib.request.Request(base + links[0], headers=headers), timeout=20) as evidence:
                        selected = evidence.read()
                        assert json.loads(selected)
                        check_retained(links[0], selected)
    logs = docker('logs', name)
    assert sentinel not in logs.stdout + logs.stderr and b'synthetic-password' not in logs.stdout + logs.stderr


def stop():
    docker('stop', '-t', '30', name)
    assert docker('inspect', '--format', '{{.State.ExitCode}}', name).stdout.strip() == b'0'
    docker('rm', name)


def copy_running(source, dest):
    # One open descriptor pins one immutable generation across publication.
    copied = run('--network', 'none', '-v', source + ':/source:ro', '-v', dest + ':/dest',
                 '--entrypoint', 'sh', a.image, '-c',
                 'exec 3</source/latest.tar; sleep 3; cat <&3 > /dest/latest.tar')
    assert copied.returncode == 0
    assert before == helper(dest, 'snapshot'), 'concurrent external-only copy changed rows'


try:
    for volume in volumes:
        docker('volume', 'create', volume)
        run('--network', 'none', '--user', '0', '-v', volume + ':/data', '--entrypoint', 'sh',
            a.image, '-c', 'chown 10001:100 /data && chmod 700 /data')
    helper(volumes[0], 'runtime')
    helper(volumes[0], 'seed')
    before = helper(volumes[0], 'snapshot')
    counts = {line.split()[0]: int(line.split()[1]) for line in before.decode().splitlines()}
    for table in ('samples', 'intervals', 'energy_aggregates', 'attempts', 'evidence',
                  'retrievals', 'jobs', 'schedules', 'completions', 'aggregate_calls'):
        assert counts[table] > 0, table
    start(volumes[0])
    # A backup timer publishes while the service remains healthy and authenticated.
    time.sleep(2)
    listing = docker('exec', name, 'find', '/data', '-maxdepth', '1', '-type', 'f').stdout
    assert b'energything.sqlite' not in listing and b'latest.tar' in listing
    mounts = json.loads(docker('inspect', name).stdout)[0]['Mounts']
    assert all(not '/var/lib/energything-runtime'.startswith(m['Destination'].rstrip('/') + '/')
               and m['Destination'] != '/var/lib/energything-runtime' for m in mounts)
    copy_running(volumes[0], volumes[1])
    stop()
    assert before == helper(volumes[0], 'snapshot'), 'first run changed retained rows'
    start(volumes[0]); stop()
    assert before == helper(volumes[0], 'snapshot'), 'replacement changed retained rows'
    # Only the externally copied dump is available in an isolated new volume.
    for source, dest in ((volumes[1], volumes[2]),):
        run('--network', 'none', '-v', source + ':/source:ro', '-v', dest + ':/dest',
            '--entrypoint', 'sh', a.image, '-c', 'cp /source/latest.tar /dest/latest.tar')
    assert before == helper(volumes[2], 'snapshot'), 'restored rows differ'
    start(volumes[2]); stop()
    assert before == helper(volumes[2], 'snapshot'), 'restored startup changed rows'
    # SIGKILL has no final export. Recreate solely from the last mounted dump.
    start(volumes[2])
    docker('kill', name)
    docker('rm', name)
    start(volumes[2]); stop()
    assert before == helper(volumes[2], 'snapshot'), 'crash recovery changed captured state'
    start(volumes[2])
    docker('exec', '--user', '0', name, 'chmod', '500', '/data')
    time.sleep(3)
    base = 'http://' + docker('port', name, '8080/tcp').stdout.decode().strip()
    request = urllib.request.Request(base + '/api/v1/status', headers={
        'Authorization': 'Basic ' + base64.b64encode(b'synthetic-user:synthetic-password').decode()})
    with urllib.request.urlopen(request, timeout=20) as response:
        state = json.load(response)['backup']
        assert state['state'] == 'error' and state['last_failure']
    docker('stop', '-t', '30', name)
    assert docker('inspect', '--format', '{{.State.ExitCode}}', name).stdout.strip() == b'1'
    docker('rm', name)
    run('--network', 'none', '--user', '0', '-v', volumes[2] + ':/data', '--entrypoint', 'chmod',
        a.image, '700', '/data')
    assert before == helper(volumes[2], 'snapshot'), 'failed final dump damaged last good state'
    run('--network', 'none', '--user', '0', '-v', volumes[3] + ':/data', '--entrypoint', 'sh',
        a.image, '-c', 'touch /data/.ownership-test && chown 0:0 /data && chmod 700 /data')
    failed = run('--network', 'none', '-v', volumes[3] + ':/data',
                 '-e', 'ENERGYTHING_USERNAME=synthetic-user', '-e', 'ENERGYTHING_PASSWORD=synthetic-password',
                 a.image, check=False)
    assert failed.returncode != 0 and b'dump restore failed' in failed.stderr + failed.stdout
    print(a.platform + ': roots/timezone, SQLite, UID, API/redaction, timer, ephemeral replacement, running external copy/isolated restore, crash, permissions and SIGTERM passed')
finally:
    docker('rm', '-f', name, check=False)
    for volume in volumes:
        docker('volume', 'rm', volume, check=False)
