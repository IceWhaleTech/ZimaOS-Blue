import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
INSTALL = ROOT / '.github/scripts/install-apple-certificate.sh'
NOTARIZE = ROOT / '.github/scripts/notarize-apple-artifact.sh'


class AppleCertificateTest(unittest.TestCase):
    def run_install(self, identity, team='TESTTEAM01', certificate='ZmFrZQ==', password='fixture-only'):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            (base / 'security').write_text('#!/bin/bash\nif [[ "$1" == find-identity ]]; then printf \'%s\\n\' "$MOCK_IDENTITY"; fi\n')
            (base / 'openssl').write_text('#!/bin/sh\nexit 0\n')
            for name in ['security', 'openssl']:
                (base / name).chmod(0o755)
            env = {**os.environ, 'PATH': str(base) + os.pathsep + os.environ['PATH'],
                   'RUNNER_TEMP': tmp, 'GITHUB_ENV': str(base / 'github.env'),
                   'CERTIFICATE_BASE64': certificate, 'CERTIFICATE_PASSWORD': password,
                   'APPLE_SIGNING_IDENTITY': '', 'APPLE_TEAM_ID': team,
                   'MOCK_IDENTITY': f'  1) HASH "{identity}"\n     1 valid identities found'}
            result = subprocess.run(['bash', str(INSTALL)], env=env, text=True, capture_output=True)
            exported = (base / 'github.env').read_text() if (base / 'github.env').exists() else ''
            return result, exported

    def test_imported_developer_id_is_exported_without_identity_secret(self):
        result, exported = self.run_install('Developer ID Application: Example (TESTTEAM01)')
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertIn('APPLE_SIGNING_IDENTITY=Developer ID Application: Example (TESTTEAM01)', exported)

    def test_development_certificate_is_rejected(self):
        result, exported = self.run_install('Apple Development: Example (TESTTEAM01)')
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertNotIn('APPLE_SIGNING_IDENTITY=', exported)

    def test_mismatched_team_is_rejected(self):
        result, exported = self.run_install('Developer ID Application: Example (OTHERTEAM1)')
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertNotIn('APPLE_SIGNING_IDENTITY=', exported)

    def test_missing_certificate_or_password_is_rejected(self):
        for certificate, password in [('', 'fixture-only'), ('ZmFrZQ==', '')]:
            with self.subTest(certificate=bool(certificate)):
                result, exported = self.run_install('Developer ID Application: Example (TESTTEAM01)',
                                                    certificate=certificate, password=password)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(exported, '')


class NotarizationTest(unittest.TestCase):
    def test_status_and_credentials(self):
        for status, expected in [('Accepted', True), ('Invalid', False), ('In Progress', False),
                                 ('malformed', False), ('missing-password', False)]:
            with self.subTest(status=status), tempfile.TemporaryDirectory() as tmp:
                base = Path(tmp)
                artifact = base / 'Blue package.zip'
                artifact.write_bytes(b'fixture archive')
                output = 'invalid JSON' if status == 'malformed' else json.dumps({'id': 'fixture-id', 'status': status})
                (base / 'xcrun').write_text(
                    '#!/usr/bin/env python3\nimport json, os, sys\n'
                    'from pathlib import Path\n'
                    'Path(os.environ["MOCK_ARGS"]).write_text(json.dumps(sys.argv[1:]))\n'
                    'print(os.environ["MOCK_RESULT"])\n')
                (base / 'xcrun').chmod(0o755)
                env = {**os.environ, 'PATH': str(base) + os.pathsep + os.environ['PATH'],
                       'RUNNER_TEMP': tmp, 'MOCK_RESULT': output, 'MOCK_ARGS': str(base / 'args.json'),
                       'APPLE_ID': 'fixture@example.invalid',
                       'APPLE_APP_PASSWORD': '' if status == 'missing-password' else 'fixture-only',
                       'APPLE_TEAM_ID': 'TESTTEAM01'}
                result = subprocess.run(['bash', str(NOTARIZE), str(artifact)], env=env, text=True, capture_output=True)
                self.assertEqual(result.returncode == 0, expected, result.stdout + result.stderr)
                if status != 'missing-password':
                    self.assertEqual(json.loads((base / 'args.json').read_text()), [
                        'notarytool', 'submit', str(artifact), '--apple-id', 'fixture@example.invalid',
                        '--password', 'fixture-only', '--team-id', 'TESTTEAM01', '--wait', '--output-format', 'json'])
                else:
                    self.assertFalse((base / 'args.json').exists())


class ReleaseWorkflowSigningTest(unittest.TestCase):
    def test_release_trigger_and_safe_preflight(self):
        text = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertIn('types: [published]', text)
        self.assertIn('workflow_dispatch:', text)
        self.assertIn("if: github.event_name == 'release'", text)
        self.assertIn('RELEASE_VERSION:', text)
        self.assertIn('preflight:', text)

    def test_imported_identity_and_both_cli_executables_are_verified(self):
        text = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertNotIn('secrets.APPLE_SIGNING_IDENTITY', text)
        self.assertIn('zip blue.zip blue bluecli', text)
        self.assertIn('codesign --verify --strict --verbose=2 bluecli', text)
        self.assertEqual(text.count('notarize-apple-artifact.sh'), 2)
        self.assertIn('xcrun stapler staple "$DMG_FILE"', text)
        self.assertIn('verify-macos-package.sh', text)

    def test_both_macos_architectures_are_checked(self):
        text = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertEqual(text.count('runner: macos-15-intel'), 2)
        self.assertEqual(text.count('runner: macos-15\n'), 2)
        self.assertIn('lipo -verify_arch "$arch" blue bluecli', text)
        self.assertIn('lipo -verify_arch "$arch" "$APP_DIR/$APP_NAME.app/Contents/MacOS/blue"', text)

    def test_ci_runs_release_regressions_without_apple_credentials(self):
        text = (ROOT / '.github/workflows/ci.yml').read_text()
        self.assertIn("python3 -m unittest discover -s scripts -p 'release_*test.py'", text)


if __name__ == '__main__':
    unittest.main()
