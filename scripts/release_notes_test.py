import json
import subprocess
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
WORKFLOW = ROOT / '.github/workflows/release.yml'


def run_step(prefix, fixture):
    step = WORKFLOW.read_text().split('      - name: ' + prefix, 1)[1]
    step = step.split('\n      - name:', 1)[0]
    script = textwrap.dedent(step.split('          script: |\n', 1)[1])
    program = '''
const script = SCRIPT;
const fixture = FIXTURE;
const calls = [];
const context = {repo: {owner: 'IceWhaleTech', repo: 'ZimaOS-Blue'},
  payload: {release: {id: 123, tag_name: '0.11.0', body: fixture.body || ''}}};
const github = {paginate: async () => [{id: 7, name: 'blue-0.11.0-darwin-arm64.tar.gz'}, {id: 8, name: 'custom-notes.txt'}], rest: {repos: {
  listReleaseAssets: () => {},
  deleteReleaseAsset: async args => calls.push(['delete', args.asset_id]),
  uploadReleaseAsset: async args => calls.push(['upload', args.name]),
  updateRelease: async args => { calls.push(['body', args.body]); context.payload.release.body = args.body; }
}}};
const fs = {readdirSync: () => ['blue-0.11.0-darwin-arm64.tar.gz'],
  statSync: () => ({isFile: () => true}), readFileSync: () => Buffer.from('fixture')};
const requireMock = name => name === 'fs' ? fs : require(name);
const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
(async () => {
  const run = new AsyncFunction('require', 'github', 'context', script);
  for (let i = 0; i < (fixture.runs || 1); i++) await run(requireMock, github, context);
  console.log(JSON.stringify({calls, body: context.payload.release.body}));
})().catch(error => { console.error(error); process.exit(1); });
'''.replace('SCRIPT', json.dumps(script)).replace('FIXTURE', json.dumps(fixture))
    result = subprocess.run(['node', '-e', program], text=True, capture_output=True, check=True)
    return json.loads(result.stdout.splitlines()[-1])


class ReleaseNotesTest(unittest.TestCase):
    def test_generated_downloads_and_zimaos_banner_are_idempotent(self):
        result = run_step('Update release body', {'body': 'Custom release notes.', 'runs': 2})
        body = result['body']
        self.assertTrue(body.startswith('Custom release notes.'))
        self.assertEqual(body.count('## Downloads'), 1)
        self.assertEqual(body.count('zimaos-banner.en.png'), 1)
        self.assertIn('utm_campaign=blue_zimaos', body)
        self.assertIn('/0.11.0/ZimaOS-Blue-0.11.0-macos-arm64.dmg', body)
        self.assertIn('/0.11.0/zimaos-blue.raw', body)

    def test_existing_localized_downloads_are_preserved(self):
        for heading in ['Downloads', '下载']:
            original = f'Custom text.\n\n## {heading}\n\n[Custom link](https://example.invalid/package)\n'
            with self.subTest(heading=heading):
                body = run_step('Update release body', {'body': original, 'runs': 2})['body']
                self.assertTrue(body.startswith(original))
                self.assertEqual(body.count('\n## '), 2)
                self.assertEqual(body.count('zimaos-banner.en.png'), 1)

    def test_upload_replaces_only_matching_prepared_asset(self):
        result = run_step('Upload assets to release', {})
        self.assertEqual(result['calls'], [['delete', 7], ['upload', 'blue-0.11.0-darwin-arm64.tar.gz']])

    def test_publication_normalizes_only_this_repositories_draft_links(self):
        body = ('## Downloads\n\n'
                '[Package](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/untagged-abc/blue.tar.gz)\n'
                '[Other](https://github.com/example/other/releases/download/untagged-abc/other.zip)\n')
        updated = run_step('Update release body', {'body': body})['body']
        self.assertIn('/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/blue.tar.gz', updated)
        self.assertIn('/example/other/releases/download/untagged-abc/other.zip', updated)


if __name__ == '__main__':
    unittest.main()
