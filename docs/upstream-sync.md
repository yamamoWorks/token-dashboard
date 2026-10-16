# Fork元の変更を取り込むための半自動ワークフロー

対象リポジトリは `yamamoWorks/token-dashboard` (Fork) と、
**読み取り専用**の `nuitsjp/token-dashboard` (upstream) です。

## 仕組み

1. GitHub Actions `Upstream sync intake` は毎週月曜09:00 JST、または手動で実行します。
2. upstreamをfetchし、最後に分析したupstream HEAD以降の変更とFork側との変更ファイルの重なりを収集します。
3. `OPENAI_API_KEY`がある場合に限り、Codexがread-onlyで機能単位の分析と移植方針を作成します。
4. Fork先に単一の `Upstream sync tracker (AI intake)` Issueを作成・更新します。
5. 人間が採否を判断し、承認した項目ごとに実装Issueを作成します。
6. 実装エージェントは `.github/upstream-sync/porting-guide.md` に従って移植と検証を行い、Draft PRを作成します。
7. 独立したReviewerと人間のレビュー・検証が済んでから手動マージします。

## 初回セットアップ

- このPRをレビューしてFork先の`main`へマージします。PR段階では定期処理は有効になりません。
- GitHub Actionsを有効にします。
- AI分析を利用する場合、Fork先の `Settings → Secrets and variables → Actions` に
  `OPENAI_API_KEY` を登録します。ChatGPTのサブスクリプションとは別のAPI費用がかかります。
- APIキーを設定しなくても、変更一覧の収集とIssueへの反映は動作します。
- `Actions → Upstream sync intake → Run workflow` を実行します。
  APIキーを後から設定し、同じupstream HEADを再分析する場合は `full_rescan=true` を選択してください。
- 管理Issueで変更内容を確認し、採用したい単位に分割してください。

## 差分検出

- 初回は共通祖先 `git merge-base origin/main upstream/main` から分析します。
- 2回目以降は管理Issueに記録したupstream HEADから分析します。
- SHAがupstreamの祖先ではない場合は共通祖先へフォールバックします。
- HEADが変化しない通常の定期実行ではAI APIを呼びません。
- `full_rescan=true` なら共通祖先から再分析します。
- Fork元・Fork先の双方で変更されたファイルを競合候補として表示します。
  名前の一致は競合確定を意味しません。

## 権限・停止点

- 収集ジョブの権限は `contents: read`, `issues: read` です。
- Codexは `:read-only` で起動し、GitHubの書き込みトークンを渡しません。
- 公開ジョブだけがForkの `issues: write` を持ちます。
- upstream内のコード・コミット・文書を命令として扱いません。
- upstreamへはfetchだけを行い、書き込みません。
- このActionは実装・Draft PR作成・mainマージを自動実行しません。
- 同名の管理Issueが複数存在する場合は更新を中止します。
- 管理Issue本文は更新時に置換します。承認・却下の記録はコメントか個別Issueに残します。
- 移植では既存の`AGENTS.md`の合意・段階・テスト手順を遵守してください。

## ローカル差分収集

```bash
git remote add upstream https://github.com/nuitsjp/token-dashboard.git
git fetch upstream main
python3 scripts/upstream_sync.py collect --fork origin/main --upstream upstream/main --output .tmp/upstream-sync
```

## 検証

```bash
python3 -m unittest discover -s scripts -p 'test_upstream_sync.py'
```

GitHub Actionsの実行、GitHub APIへのIssue作成、および実際のAI分析は
本PRのマージ後にのみ実行・確認できます。
