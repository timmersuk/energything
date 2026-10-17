#!/usr/bin/env python3
"""Inspect actual allowlisted context and every saved image layer, without private output."""
import argparse
import io
import pathlib
import subprocess
import tarfile
import tempfile
import uuid

p = argparse.ArgumentParser()
p.add_argument('--image', required=True)
a = p.parse_args()
sentinel = ('TASK07-EXCLUDED-' + uuid.uuid4().hex).encode()
paths = [pathlib.Path(d) / ('task07-' + uuid.uuid4().hex) for d in ('.cache', 'private', 'backups')]
try:
    for path in paths:
        path.parent.mkdir(exist_ok=True)
        path.write_bytes(sentinel)
    with tempfile.TemporaryDirectory() as tmp:
        subprocess.run(['docker', 'buildx', 'build', '--no-cache', '-f', '-',
                        '--output', 'type=local,dest=' + tmp, '.'],
                       input=b'FROM scratch\nCOPY . /context/\n', check=True,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        root = pathlib.Path(tmp) / 'context'
        expected = {'Dockerfile'} | {f'bin/linux/{arch}/{binary}'
                    for arch in ('amd64', 'arm64') for binary in ('energything', 'energything-enqueue')}
        actual = {str(f.relative_to(root)).replace('\\', '/') for f in root.rglob('*') if f.is_file()}
        # BuildKit can consume/remove Dockerfile when it is supplied on stdin.
        assert actual in (expected, expected - {'Dockerfile'}), 'build context differs from explicit allowlist'
        for f in root.rglob('*'):
            if f.is_file():
                assert sentinel not in f.read_bytes()
    with tempfile.TemporaryFile() as saved:
        subprocess.run(['docker', 'save', a.image], stdout=saved, check=True)
        saved.seek(0)
        with tarfile.open(fileobj=saved) as archive:
            for member in archive:
                if member.isfile():
                    data = archive.extractfile(member).read()
                    assert sentinel not in data, 'excluded sentinel in image layer'
                    # docker save may use classic layers or OCI content blobs.
                    try:
                        with tarfile.open(fileobj=io.BytesIO(data)) as layer:
                            for entry in layer:
                                path = entry.name.removeprefix('./')
                                assert not (path.startswith(('context/', 'private/', 'backups/', '.cache/'))
                                            or path.endswith(('.sqlite', '.sqlite-wal', '.sqlite-shm'))
                                            or path.rsplit('/', 1)[-1] == '.env'), 'private/runtime path in layer'
                    except tarfile.ReadError:
                        pass
    print('Actual build context allowlist and all saved image layers passed private-file exclusion')
finally:
    for path in paths:
        path.unlink(missing_ok=True)
