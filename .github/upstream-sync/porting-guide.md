# Upstream機能の移植：実装エージェントへの指示

以下を人間が選定・承認した後、Fork先の作業ブランチでCodexへ渡します。

- Fork先の対象Issue番号
- upstreamの対象コミットSHA（単体または複数）
- 人間が承認した受け入れ条件

## AI Implementer

`yamamoWorks/token-dashboard` の承認済みIssueを対象に、
upstreamの変更意図と期待動作を確認し、Fork先の現在のアーキテクチャへ移植する。

1. `AGENTS.md` と該当する仕様・設計・テストを確認し、同意を必要とする段階では停止する。
2. Forkの独自機能（3.5インチ Rev.A、ページング、Windowsロック・ディスプレイ電源連動）を維持する。
3. Cherry-pickは独立したコミットに限り、構造が異なる箇所は意味的に移植する。
4. 対象Issue以外のリファクタリング、上流の規約上書き、mainへの直接変更をしない。
5. 必要な回帰テストを追加し、可能な検証を実施する。
6. 検証結果と未確認事項を示し、作業ブランチからDraft PRを作成する。

## AI Reviewer

Implementerとは独立して、変更意図・後方互換性・デバイス固有機能・
依存関係・CI・仕様/文書整合性をリスクベースで検証する。
検証していない項目を合格とみなさない。

## 禁止事項

- upstream (`nuitsjp/token-dashboard`) へのIssue作成、push、PR作成その他の書き込み
- Fork先mainへの自動マージ
- 人間が採用を承認していないupstream変更の実装

Issue、コミット範囲または受け入れ条件が未確定なら実装に着手しない。
