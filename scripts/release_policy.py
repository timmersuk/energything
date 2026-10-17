"""Final pre-login release validation; never accepts a manual push switch."""
import json
import os
import re


def release_version(event_name, event):
    release = event.get('release', {})
    tag = release.get('tag_name', '')
    if (event_name != 'release' or event.get('action') != 'published'
            or release.get('draft', True) or release.get('prerelease', True)
            or not re.fullmatch(r'v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)', tag)):
        raise ValueError('Only published stable vMAJOR.MINOR.PATCH releases may publish')
    return tag


if __name__ == '__main__':
    with open(os.environ['GITHUB_EVENT_PATH'], encoding='utf-8') as f:
        tag = release_version(os.environ['GITHUB_EVENT_NAME'], json.load(f))
    if not re.fullmatch(r'[a-z0-9_-]+/energything', os.environ['DOCKERHUB_IMAGE']):
        raise ValueError('Set DOCKERHUB_IMAGE to namespace/energything')
    build = f"{tag}-sha-{os.environ['GITHUB_SHA']}-run-{os.environ['GITHUB_RUN_ID']}-{os.environ['GITHUB_RUN_ATTEMPT']}"
    with open(os.environ['GITHUB_ENV'], 'a', encoding='utf-8') as f:
        f.write(f'BUILD_ID={build}\n')
