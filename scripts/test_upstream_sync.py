"""Regression tests for read-only upstream inventory (no GitHub network)."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

import upstream_sync

def cmd(directory, *args):
    return subprocess.check_output(['git', '-C', str(directory), *args], text=True).strip()

def commit(repo, filename, body):
    (repo / filename).write_text(body, encoding='utf-8')
    cmd(repo, 'add', filename)
    cmd(repo, 'commit', '-m', 'change')
    return cmd(repo, 'rev-parse', 'HEAD')

class InventoryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        cmd(self.repo, 'init', '-b', 'main')
        cmd(self.repo, 'config', 'user.email', 'test@example.test')
        cmd(self.repo, 'config', 'user.name', 'Integration Test')
        commit(self.repo, 'shared.txt', 'common')
        cmd(self.repo, 'branch', 'upstream')
        commit(self.repo, 'shared.txt', 'fork change')
        cmd(self.repo, 'checkout', 'upstream')
        self.first = commit(self.repo, 'shared.txt', 'upstream change')
        self.second = commit(self.repo, 'feature.txt', 'upstream feature')
        cmd(self.repo, 'checkout', 'main')

    def scan(self, since='', full=False):
        args = type('Args', (), {
            'fork': 'main', 'upstream': 'upstream', 'since': since,
            'full': full, 'output': str(self.repo / 'report'),
        })()
        previous = os.getcwd()
        try:
            os.chdir(self.repo)
            upstream_sync.collect(args)
        finally:
            os.chdir(previous)
        return json.loads((self.repo / 'report/metadata.json').read_text())

    def test_first_scan_identifies_divergence(self):
        result = self.scan()
        self.assertEqual(result['new_commit_count'], 2)
        self.assertEqual(result['overlapping_path_count'], 1)

    def test_marker_avoids_duplicate_analysis(self):
        self.assertEqual(self.scan(self.second)['new_commit_count'], 0)

    def test_marker_only_includes_new_commits(self):
        result = self.scan(self.first)
        self.assertEqual(result['new_commit_count'], 1)
        self.assertTrue(result['used_previous'])

    def test_full_rescan_ignores_marker(self):
        self.assertEqual(self.scan(self.second, full=True)['new_commit_count'], 2)

    def test_invalid_marker_falls_back_to_common_ancestor(self):
        result = self.scan('f' * 40)
        self.assertEqual(result['new_commit_count'], 2)
        self.assertFalse(result['used_previous'])

if __name__ == '__main__':
    unittest.main()
