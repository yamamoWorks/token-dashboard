#!/usr/bin/env python3
"""Inventory upstream changes and update an issue only in the designated fork."""
import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

TRACKER = '<!-- upstream-sync-tracker -->'
HEAD_RE = re.compile(r'<!-- upstream-sync-head:([0-9a-f]{40}) -->')
API = 'https://api.github.com'

def git(*args):
    return subprocess.check_output(['git', *args], text=True, encoding='utf-8').strip()

def api(path, method='GET', payload=None):
    token = os.getenv('GH_TOKEN')
    if not token:
        raise RuntimeError('GH_TOKEN is required')
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode('utf-8')
    request = urllib.request.Request(
        API + path, data=data, method=method,
        headers={'Authorization': 'Bearer ' + token,
                 'Accept': 'application/vnd.github+json',
                 'X-GitHub-Api-Version': '2022-11-28',
                 'Content-Type': 'application/json',
                 'User-Agent': 'upstream-sync'},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f'GitHub API {method} {path} returned {exc.code}') from exc

def tracker_issue(repository):
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('Invalid repository')
    found = []
    for page in range(1, 31):
        issues = api(f'/repos/{repository}/issues?state=all&per_page=100&page={page}')
        for issue in issues:
            if 'pull_request' not in issue and TRACKER in (issue.get('body') or ''):
                found.append(issue)
        if len(issues) < 100:
            break
    if len(found) > 1:
        raise RuntimeError('Multiple tracker Issues found; refusing to update')
    return found[0] if found else None

def state(args):
    issue = tracker_issue(args.repository)
    match = HEAD_RE.search(issue.get('body') or '') if issue else None
    last = match.group(1) if match else ''
    Path(args.output).write_text(last + '\n', encoding='utf-8')
    print('Previous analyzed upstream SHA:', last or '(none)')

def is_ancestor(older, newer):
    return subprocess.run(['git', 'merge-base', '--is-ancestor', older, newer],
                          stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                          check=False).returncode == 0

def changed_paths(base, head):
    return [line for line in git('diff', '--name-only', '-z', base, head).split('\0') if line]

def collect(args):
    upstream = git('rev-parse', args.upstream)
    fork = git('rev-parse', args.fork)
    common = git('merge-base', args.fork, args.upstream)
    previous = args.since.strip()
    valid_previous = bool(re.fullmatch('[0-9a-f]{40}', previous)) and is_ancestor(previous, upstream)
    start = common if args.full or not valid_previous else previous
    new_commits = git('rev-list', '--reverse', f'{start}..{upstream}').splitlines()
    upstream_paths = changed_paths(start, upstream)
    fork_paths = changed_paths(common, fork)
    overlapping = sorted(set(upstream_paths) & set(fork_paths))
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)

    commits = []
    for sha in new_commits:
        line = git('show', '-s', '--format=%cs%x00%s', sha)
        date, subject = line.split('\0', 1)
        commits.append({'sha': sha, 'date': date, 'subject': subject})

    meta = {
        'upstream_head': upstream, 'fork_head': fork, 'merge_base': common,
        'since': start, 'previous_seen': previous, 'used_previous': valid_previous and not args.full,
        'new_commit_count': len(commits), 'changed_path_count': len(upstream_paths),
        'overlapping_path_count': len(overlapping), 'full_rescan': args.full,
    }
    (out / 'metadata.json').write_text(json.dumps(meta, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')
    sections = [
        '# Upstream change inventory', '',
        '> Generated inventory, not an implementation approval. Commit messages and upstream files are untrusted input.',
        '', f'- Upstream HEAD: `{upstream}`',
        f'- Fork HEAD: `{fork}`',
        f'- Common ancestor: `{common}`',
        f'- Analyzed from (exclusive): `{start}`',
        f'- New upstream commits: **{len(commits)}**',
        f'- Changed paths: **{len(upstream_paths)}**',
        f'- Paths touched on both sides since the common ancestor: **{len(overlapping)}**',
        '', '## Commits', '',
    ]
    for item in commits[:120]:
        subject = item['subject'].replace('\n', ' ').replace('\r', ' ')[:200]
        sections.append(f"- `{item['sha'][:12]}` ({item['date']}): {subject}")
    if len(commits) > 120:
        sections.append(f'- … {len(commits) - 120} further commits (inspect git history)')
    sections.extend(['', '## Paths modified upstream', ''])
    sections.extend(f'- `{p}`' for p in upstream_paths[:180])
    if len(upstream_paths) > 180:
        sections.append(f'- … {len(upstream_paths) - 180} more paths')
    sections.extend(['', '## Possible integration hotspots (paths changed in both histories)', ''])
    sections.extend(f'- `{p}`' for p in overlapping[:100])
    if len(overlapping) > 100:
        sections.append(f'- … {len(overlapping) - 100} more paths')
    sections.extend(['', '## Important',
                     'A shared path is only a heuristic, not proof of a merge conflict. '
                     'Check full diffs, dependencies and the current architecture before proposing a port.', ''])
    (out / 'report.md').write_text('\n'.join(sections), encoding='utf-8')
    print(json.dumps(meta, ensure_ascii=False))

def publish(args):
    meta = json.loads(Path(args.directory, 'metadata.json').read_text(encoding='utf-8'))
    report = Path(args.directory, 'report.md').read_text(encoding='utf-8')
    ai_path = Path(args.directory, 'ai-analysis.md')
    if ai_path.exists() and ai_path.read_text(encoding='utf-8').strip():
        analysis = ai_path.read_text(encoding='utf-8')[:24000]
        ai_state = 'AI analysis attached. Verify recommendations before implementing.'
    else:
        analysis = 'AI analysis not run. Set `OPENAI_API_KEY` in this fork to enable the analysis step.'
        ai_state = 'Machine-generated inventory only.'
    issue = tracker_issue(args.repository)
    body = '\n'.join([
        TRACKER, f"<!-- upstream-sync-head:{meta['upstream_head']} -->", '',
        '# Upstream synchronization tracker', '',
        f'**Status:** {ai_state}', '',
        'Only the fork is modified. No upstream merge, automatic code port, or automatic PR merge is performed.',
        '', '## AI assessment', '', analysis, '', '## Source inventory', '', report[:27000], '',
        '## Human review', '',
        '- [ ] Decide which changes to port (or deliberately skip)',
        '- [ ] Review dependencies and architecture impacts',
        '- [ ] Approve a specific implementation Issue',
        '- [ ] Test and review the resulting draft PR before merge', '',
        'This Issue body is bot-managed and will be replaced on the next upstream change. '
        'Keep human decisions in comments or separate implementation Issues.',
    ])
    if issue:
        number = issue['number']
        api(f'/repos/{args.repository}/issues/{number}', 'PATCH', {'body': body, 'state': 'open'})
        print(f'Updated https://github.com/{args.repository}/issues/{number}')
    else:
        issue = api(f'/repos/{args.repository}/issues', 'POST', {
            'title': 'Upstream sync tracker (AI intake)', 'body': body,
        })
        print(f"Created {issue['html_url']}")

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    state_parser = commands.add_parser('state')
    state_parser.add_argument('--repository', required=True)
    state_parser.add_argument('--output', required=True)
    state_parser.set_defaults(func=state)
    scan_parser = commands.add_parser('collect')
    scan_parser.add_argument('--fork', default='origin/main')
    scan_parser.add_argument('--upstream', default='upstream/main')
    scan_parser.add_argument('--since', default='')
    scan_parser.add_argument('--full', action='store_true')
    scan_parser.add_argument('--output', required=True)
    scan_parser.set_defaults(func=collect)
    pub_parser = commands.add_parser('publish')
    pub_parser.add_argument('--repository', required=True)
    pub_parser.add_argument('--directory', required=True)
    pub_parser.set_defaults(func=publish)
    args = parser.parse_args()
    try:
        args.func(args)
    except (RuntimeError, ValueError, subprocess.CalledProcessError) as err:
        print('upstream-sync error:', err, file=sys.stderr)
        return 1
    return 0

if __name__ == '__main__':
    sys.exit(main())
