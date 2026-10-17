import unittest
from release_policy import release_version


class ReleasePolicyTest(unittest.TestCase):
    def test_event_matrix(self):
        valid = {'action': 'published', 'release': {'tag_name': 'v1.2.3', 'draft': False, 'prerelease': False}}
        self.assertEqual(release_version('release', valid), 'v1.2.3')
        for event in ('pull_request', 'pull_request_target', 'push', 'workflow_dispatch'):
            with self.subTest(event=event), self.assertRaises(ValueError):
                release_version(event, valid)
        for field, value in (('draft', True), ('prerelease', True), ('tag_name', 'main'),
                             ('tag_name', 'v1.2.3-rc1'), ('tag_name', 'v01.2.3'), ('tag_name', 'v1.2.3\n')):
            event = {**valid, 'release': {**valid['release'], field: value}}
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                release_version('release', event)
        for action in ('created', 'edited', 'unpublished'):
            with self.subTest(action=action), self.assertRaises(ValueError):
                release_version('release', {**valid, 'action': action})
        with self.assertRaises(ValueError):
            release_version('release', {})
