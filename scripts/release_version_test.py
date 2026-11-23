import json
import os
import re
import tomllib
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
VERSION = (ROOT / 'VERSION').read_text().strip()


class ReleaseVersionTest(unittest.TestCase):
    def test_event_and_native_versions_match_source(self):
        self.assertRegex(VERSION, r'^\d+\.\d+\.\d+$')
        self.assertEqual(os.environ.get('RELEASE_VERSION', VERSION).removeprefix('v'), VERSION)
        go = (ROOT / 'server/internal/buildinfo/version.go').read_text()
        self.assertEqual(re.search(r'const Version = "([^"]+)"', go).group(1), VERSION)
        make = (ROOT / 'Makefile').read_text()
        self.assertEqual(re.search(r'^VERSION \?= (\S+)', make, re.M).group(1), VERSION)
        for name in ['blue', 'bluelib']:
            source = (ROOT / f'server/cmd/{name}/main.go').read_text()
            self.assertRegex(source, r'version\s*=\s*buildinfo\.Version')

    def test_javascript_manifests_and_locks_match_source(self):
        for name in ['web', 'tauri-app']:
            with self.subTest(package=name):
                package = json.loads((ROOT / name / 'package.json').read_text())
                lock = json.loads((ROOT / name / 'package-lock.json').read_text())
                self.assertEqual(package['version'], VERSION)
                self.assertEqual(lock['version'], VERSION)
                self.assertEqual(lock['packages']['']['version'], VERSION)

    def test_tauri_and_rust_metadata_match_source(self):
        base = ROOT / 'tauri-app/src-tauri'
        config = json.loads((base / 'tauri.conf.json').read_text())
        cargo = tomllib.loads((base / 'Cargo.toml').read_text())
        lock = tomllib.loads((base / 'Cargo.lock').read_text())
        self.assertEqual(config['version'], VERSION)
        self.assertEqual(cargo['package']['version'], VERSION)
        package = next(p for p in lock['package'] if p['name'] == cargo['package']['name'])
        self.assertEqual(package['version'], VERSION)


if __name__ == '__main__':
    unittest.main()
