# Compact Rev.A版：Fork元の変更を取り込む運用手順

## 1. 目的・適用範囲

この文書は、[yamamoWorks/token-dashboard](https://github.com/yamamoWorks/token-dashboard) の **3.5インチ TURZX Rev.A専用版（案B）** に、Fork元 [nuitsjp/token-dashboard](https://github.com/nuitsjp/token-dashboard) の変更を継続的に取り込む手順を定める。

- **書き込みを許可するリポジトリは `yamamoWorks/token-dashboard` のみ。Fork元 `nuitsjp/token-dashboard` は参照専用とし、ブランチ・Issue・PR・タグ・コミット等を一切書き込まない。**
- 同期作業は専用ブランチで行い、レビュー・検証後にFork先の `main` へPRで反映する。Fork元の「Sync fork」をFork先 `main` へ直接適用しない。
- 旧実装を保存した `legacy/3.5-go-renderer` は**履歴・参照用**であり、今後のFork元同期にマージしない。
- 9.2インチ対応、Windowsロック・ディスプレイ電源OFF・REST制御は現行Compact版の対象外。同期によって意図せず機能を復活・混入させない。
- 本書は利用者が指定したFork専用の手順書として追加したもの。Fork元の [文書方針](document-policy.md) は新規の独立した手順書を原則禁止しているため、文書体系上の整合は別途検討事項とする。**本書の追加を理由にFork元由来の文書を改変しない。**

## 2. 同期時に守る実装境界

Fork元の既存ファイルを修正するのは、原則として次の **2ファイル** に限定する。

| 既存ファイル | Fork側の差分 | 更新時の確認 |
| --- | --- | --- |
| `main.go` | Compact Mirror / Bridge / Rev.A Outputの結線、更新無効化 | Fork元の起動処理を反映し、Compact結線と更新遮断を維持する |
| `frontend/vite.config.ts` | `compactModulePlugin()` の登録 | Fork元のVite設定を反映し、プラグイン適用を維持する |

Compact固有の実装は原則として新規ファイルに閉じ込める。

- `internal/compact/**`：Mirror、Bridge、PNG左上480×320切り出し、RGB565LE短冊、Rev.A Serial、関連テスト
- `frontend/src/compact/**`：Compact専用ルート・プレビュー・設定UIとモジュール差し替えプラグイン
- `themes/compact/**`：1920×462の仮想Canvasで左上480×320を描画するテーマ
- `frontend/tests/compact-e2e/**`、`frontend/playwright.compact.config.ts`：Compact専用E2E
- `scripts/compact-verify.mjs`、`.github/workflows/compact-reva.yml`：Compact専用検証

`internal/display/**`、Fork元の既存React画面、既存Goテスト、`frontend/tests/e2e/**`、`Taskfile.yml`、`build/app.json`、既存の `scripts/run.mjs` / `scripts/build.mjs` / Windows CIは、**原則Fork元と同じ内容**を保つ。新しいFork元側の変更は取り込み、Fork独自のパッチをこれらに増やさない。

「既存2ファイル」の計測は、**新たに取り込んだ `upstream/main` のツリーとの比較**で行う。Fork側の追加ファイルは別件数として集計する。将来Fork元がCompactと同じパスのファイルを追加した場合は衝突として明示的に判断する。

### 変更してはいけない動作契約

1. `usage.State` は全データを保持し、`compact.Mirror` が非表示設定・リスク順・最大2窓・2プロバイダー単位の**10秒固定**ページ送りを担う。`display.Run` 側で二重に巡回させない。
2. `display.Run` と `CanvasRenderer` はFork元のものを利用する。通常JPEGは `compact.Bridge.OnOutput` が破棄し、`display.Service.Preview()` に格納された **PNG** を `display:updated` に合わせて `Bridge.OnUpdated` が取得する。離席時は全黒JPEGのみ黒画面指示として扱う。
3. 通常転送は **PNG → 左上480×320 → RGB565LE → 8px幅×60短冊 → Rev.A Serial**。再接続、最新フレーム優先、復帰、終了時RESETを維持する。
4. `CompactModulePlugin` はFork元の `routes/index.tsx`、`routes/settings.tsx`、`StyleList.tsx` をビルド時に代替する。対象のimport/export/ルーティングが変わった場合は、元のReactファイルを編集せずに**新規Compact実装・プラグイン側を追従**させる。
5. 上流リリースへの自動更新は無効のままにする。埋め込み `build/app.json` の更新URL/公開鍵と、ビルド時 `BUILD_UPDATE_SOURCE` / `BUILD_UPDATE_PUBLIC_KEY` の両方を、実行時に無効化できていることを確認する。

## 3. 取り込み手順

以下はローカルGitで実施する例。**push先は必ず `origin`（`yamamoWorks/token-dashboard`）とし、`upstream` へpushしない。**

### 3.1. リモートと変更内容の確認

```bash
git remote -v
# upstream が未登録の場合のみ:
git remote add upstream https://github.com/nuitsjp/token-dashboard.git

git fetch origin
git fetch upstream main
git log --oneline main..upstream/main
git rev-parse upstream/main
```

- `origin` が `yamamoWorks/token-dashboard`、`upstream` が `nuitsjp/token-dashboard` であることを必ず確認する。
- `upstream/main` が現行 `main` の祖先であれば新規の取り込みは不要（`git merge-base --is-ancestor upstream/main main`）。
- Fork元に履歴の書き換えがあった場合は、通常同期を自動続行せず、差分・共通祖先・影響を確認して別途対処を決める。
- 更新対象のFork元コミットの40桁SHAをPRに記録し、変更内容をGo描画、API、UI、ビルド、依存関係、CI、アップデータに分類して評価する。

### 3.2. Fork先mainから同期ブランチを作成

ローカル変更は先にコミットまたは退避する。ブランチ名の日付は実施日に置き換える。

```bash
git switch main
git pull --ff-only origin main
git switch -c sync/upstream-YYYYMMDD
git merge --no-ff upstream/main
```

- **通常マージ（merge commit）を標準**とする。Fork元履歴の祖先関係を保存するため、同期ブランチを `main` に取り込むPRも **Create a merge commit** を選ぶ。
- **Squash merge / Rebase mergeは原則しない**。Fork元コミットの履歴が失われ、次回同期時に同じ変更の再取り込みや競合を招き得る。
- 競合時には作業を停止してファイル単位で解消する。Fork元の新しいロジックを取り込み、Compact固有の結線のみ適用し直す。`--ours` / `--theirs` で一括解決しない。
- 互換性が取れなければ `git merge --abort` で中断し、PRへ未完了理由を記録する。`main` に直接コミット・強制pushをしない。

### 3.3. 重点互換性レビュー

| 更新された箇所 | 必須確認事項 |
| --- | --- |
| `main.go` とWails API | 起動、サービス登録、イベント発行、終了・キャンセルの変更を取り込んだか |
| `internal/display/service.go`、`canvas.go` | `frame.JPEG`、`Preview()` のPNG、`display:updated` の順序・通知契約が変わっていないか |
| `internal/usage/**`、`internal/settings/**` | Mirrorのデータ入力、非表示・ページ巡回、ローカル/Hub切替の互換性 |
| `frontend/vite.config.ts` とルート群 | Vite / TanStack Routerの構成、モジュールID、export、Compact置換が正しいか |
| `frontend/src/features/display/**`、テーマ関連 | Canvasサイズ・PNG・テンプレート入力・テーマ生成手順の変更への追従 |
| `scripts/**`、`build/app.json`、`.github/workflows/**` | ビルド/リリース経路、バージョン、更新URL・公開鍵、テストの実行対象が変わっていないか |
| 依存関係 | Go、Node、Wails、Vite、Playwright更新による互換性・生成物の変化 |

既存ルートをそのまま残すだけでなく、**Compact画面が実際に本番バンドルへ含まれていること**を確認する。現在の `CompactModulePlugin` は対象パス完全一致・置換漏れエラー・生成コードのマーカー検査を行うが、動作検証の代わりにはならない。

### 3.4. Compact CI内のFork元固定SHAを更新

`.github/workflows/compact-reva.yml` の **Preserve upstream tests and build entry points** にある `git fetch` 対象の40桁SHAを、**今回実際に取り込んだ `upstream/main` のSHA**へ更新する。

- 現行の初期基準値：`15c8447272a3faf16749e6d07f14c9f4d6a60b30`（2026-10-19時点）。以後の同期では値の据え置きを前提にしない。
- CIはこのSHAを使って既存E2E、Playwright設定、Goテスト、Taskfile、build設定、元のWindows workflowがFork元と同一であることを検査する。
- SHAを変更してテストを回避するのではなく、まずFork元の新しい既存ファイルと一致させたうえで更新する。
- `compact-reva.yml` はFork側の追加ファイルなので、SHA更新は既存ファイルの「2件修正目標」に算入しない。

## 4. 検証とPRの受け入れ条件

### 4.1. ローカル・Windowsでの検証

```bash
node scripts/run.mjs setup
node scripts/compact-verify.mjs verify
# NSISを導入したWindows環境で実施
node scripts/compact-verify.mjs ci
```

実施環境のNodeは `.nvmrc` と一致させ、Playwright Chromiumがなければ `npm --prefix frontend exec -- playwright install --only-shell chromium` で準備する。

- Go root/internal/releaseのテスト、vet、TS lint/typecheck、Vitest、文書検査、Compact E2Eを実行する。
- `ci` ではWindowsデスクトップ・NSISインストーラーの生成、埋め込みCompact資産の確認まで実施する。
- [Compact専用GitHub Actions](../.github/workflows/compact-reva.yml) が対象PRの**更新後HEAD**で成功したことを確認する。失敗・skip・未実行はそのまま記録する。
- PNG切出し、黒画面・復帰、Rev.A再接続、固定10秒ページ巡回、480×320プレビュー、自動更新遮断、Local/Hub・非表示設定を検証する。
- シリアル制御・描画・接続方式に影響する場合は **3.5インチRev.A実機**で送信、視認性、抜き差し、RESETを確認する。COMポートのアクセス拒否等で実機未検証なら「完了」と扱わず残件を明示する。

### 4.2. 従来のCI・リリースの扱い

Fork元の `.github/workflows/windows.yml` は旧9.2インチ向けE2Eを含み、現在のCompact版の仕様とは一致しないテストがある。**Compact CIの成功を「標準CIを含む全件合格」と表現しない。**

- 従来CIの失敗・skip・成功をCompact CIと**別々に**PRへ記録する。無断で既存テスト、workflow、required checkを改変・無効化しない。
- `.github/workflows/release.yml` は現在 `node scripts/run.mjs verify` を呼ぶため、旧E2Eの不整合が残る状態でタグ発行・リリースを成功前提に進めない。
- Fork先独自のリリース・更新チャネルが安全に確立するまでは、**上流自動更新の無効化を維持**する。Fork元の更新機構や署名設定が変わったときは必ず影響を確認する。

### 4.3. 差分を計測し、PRへ記録

```bash
# 取り込んだFork元とFork先候補のツリー差分
git diff --name-status upstream/main HEAD
git diff --stat upstream/main HEAD
```

原則の期待値は、既存ファイルの `M main.go` と `M frontend/vite.config.ts`、およびFork側の `A`（新規追加）のみ。

PRには次を残す。

- 取り込み元SHA、同期前Fork先SHA、同期後候補SHA、取り込んだFork元コミット一覧
- 既存ファイル変更数・変更パス、新規追加数、目標2件を超えた場合の例外理由
- 契約変更への対応、Compact固有コードの追加・修正箇所
- Compact/従来CIのそれぞれのURL・合否、実機の検証結果・未確認事項
- ロールバック先のコミット、マージ・復旧手順

**PRマージ条件:** Compactの必要なテスト・ビルドが成功し、機能・差分・安全性・標準CIの既知問題をレビューできていること。実機に影響するのに未検証の場合は、そのリスクについて利用者の明示判断を得ること。

## 5. Fork先mainへの反映・ロールバック

1. `sync/upstream-YYYYMMDD` を `origin` にpushする。
2. **`yamamoWorks/token-dashboard` の `main` 宛てにPR**を作る。Fork元へPR・pushしない。
3. レビューと受け入れ条件を満たしてから、**Create a merge commit** でマージする。通常の同期では `main` のforce-pushや履歴の作り直しをしない。
4. マージ後の `main` でCompact CI・差分・ビルド結果を再確認する。

不具合が見つかった場合は、同期前の `main` SHAを参照し、**ロールバック用PR**を用意する。PRが単一のマージコミットであれば、そのコミットを対象に `git revert -m 1 <merge-commit-sha>` を検討できる。ただし、このRevertは**後の同一Fork元変更の再取り込みにも影響する**ため、復旧対象の範囲と次回の再マージ戦略を確認する。通常は `git reset --hard` や `main` へのforce-pushでは戻さない。

## 6. 更新しない場合・例外

- Fork元が更新されていない場合は同期不要。
- Fork元の変更がCompact専用版に不要・有害な場合、原則はFork元の履歴を維持しつつCompact側で無効化・適応する。保持が危険なら同期を保留し、対象コミットと理由をIssue/PRに記録する。
- 元ファイルを3件以上修正しないと成立しない場合、差分増加を既成事実化せず、まずFork側の新規アダプター・ビルド時の差し替えで吸収できるか検討する。それでも必要なら変更ファイル・理由・影響を示して合意を得る。
- upstreamが既存のCompact専用パスを追加・改名した場合は名前衝突をレビューする。機械的に上書きしない。

---

**関連**：全体管理 [Issue #24](https://github.com/yamamoWorks/token-dashboard/issues/24) ／ 案B移行 [#40](https://github.com/yamamoWorks/token-dashboard/issues/40)・[#41](https://github.com/yamamoWorks/token-dashboard/issues/41)・[#42](https://github.com/yamamoWorks/token-dashboard/issues/42)・[#43](https://github.com/yamamoWorks/token-dashboard/issues/43) ／ main切替 [#33](https://github.com/yamamoWorks/token-dashboard/issues/33)。
