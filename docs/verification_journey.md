# Dewy ゼロダウンタイム自動切り替え検証ジャーニー

## 1. はじめに・検証の目的

本検証は、Go アプリケーションのデプロイメントツール **[Dewy](https://github.com/linyows/dewy)** を用い、**「オブジェクトストレージ（さくらのクラウド オブジェクトストレージ）経由でのバイナリ自動配信と、ゼロダウンタイム自動切り替え（Graceful Restart）」** をローカル開発環境および CI/CD（GitHub Actions）で完全に自動化・検証することを目的として実施されました。

---

## 2. 直面した課題と技術的解決

### 課題 1: SOPS 暗号化シークレットと CLI 実行環境の乖離
- **現象**:
  `./bin/dewyctl release --version 0.0.1` 実行時に `Error: AWS_ACCESS_KEY_ID is not configured` が発生。
- **原因**:
  認証情報は `secrets.enc.yml` に SOPS で安全に暗号化保存されていたが、`release` や `push`、`doctor` コマンドがシェルの環境変数（`os.Getenv`）のみを直接参照しており、暗号化ファイルからの自動復号・ロード機構が一部で不足していた。
- **解決策**:
  `sops.LoadCredentials()` を共通化し、環境変数が未設定でも `secrets.enc.yml` から透過的にクレデンシャル（S3 バケット、エンドポイント、アクセスキー、シークレットキー）を復号してプロセス環境変数に自動注入するように改修。

---

### 課題 2: プラットフォーム（OS / Arch）の不一致による更新無視
- **現象**:
  ローカル macOS（Apple Silicon）上で `./bin/dewyctl server` を起動した状態で、`./bin/dewyctl release --version 0.0.2` を実行したところ、S3 にアップロードは完了したものの、Dewy 側が新バージョンを全く認識しなかった。
- **原因**:
  - **Dewy サーバー側（ホストが macOS）**:
    ホストのアーキテクチャに合わせて `Target Artifact: dewy_practice_darwin_arm64.tar.gz` を S3 上で監視していた。
  - **リリース側（dewyctl release）**:
    GoReleaser 未導入時のフォールバック処理において、デフォルトで `linux/amd64`（`dewy_practice_linux_amd64.tar.gz`）のみをビルド・アップロードしていた。
  - **結果**:
    S3 上に `darwin/arm64` 向けのバイナリが存在しなかったため、Dewy は「自分用のバイナリではない」と判断し、新バージョンを無視してスキップしていた。
- **解決策**:
  - **マルチプラットフォーム同時ビルド & アップロード**:
    GoReleaser が本来行う挙動と同様に、`dewyctl release`（および `dewyctl push`）で、以下の **4 プラットフォームを一括クロスコンパイルして S3 へ同時アップロード** するように改修：
    1. `linux/amd64`（Linux 本番サーバー / Docker 用）
    2. `linux/arm64`（Linux ARM / AWS Graviton 用）
    3. `darwin/arm64`（Apple Silicon Mac / ローカル検証用）
    4. `darwin/amd64`（Intel Mac 用）
  - **成果物名の一貫性確保**:
    `.goreleaser.yaml` 内の `project_name`（`dewy-app`）を自動検出し、サーバー側とリリース側でプレフィックス命名（`dewy-app_<os>_<arch>.tar.gz`）が 100% 一致するよう統一。

---

## 3. ゼロダウンタイム切り替えの実証ログ

修正後、`./bin/dewyctl server` をフォアグラウンド実行した状態で、別ターミナルから `./bin/dewyctl release --version 0.0.4` を実行した際の実ログです：

```text
============================================================
           Starting Dewy Server Daemon
============================================================
 Host Platform:   darwin/arm64
 Target Artifact: dewy-app_darwin_arm64.tar.gz
 Registry URL:    s3://jp-east-1/to-shoji-dewy-bucket/app?endpoint=https://s3.tky01.sakurastorage.jp&artifact=dewy-app_darwin_arm64.tar.gz
 Application Port:8080
 Polling Interval:5s
 Working Dir:     /Users/shjtmy/gravity/dewy_practice/.dewy
 Binary Target:   /Users/shjtmy/gravity/dewy_practice/.dewy/current/dewy-app
============================================================
Dewy is now polling Object Storage for new releases. Press Ctrl+C to stop.

[Step 1: 初期バージョン v0.0.3 の起動]
time=2026-09-21T23:58:24.322+09:00 level=INFO msg="starting new worker" pid=29033
2026/09/21 23:58:24 Starting Dewy App (Version: v0.0.3)...
2026/09/21 23:58:24 Running under server-starter. Inheriting listening socket.
2026/09/21 23:58:24 Server listening on 0.0.0.0:8080

[Step 2: release v0.0.4 の検知と自動切り替え (Zero-Downtime)]
time=2026-09-21T23:59:04.282+09:00 level=INFO msg="received HUP signal" num_old_workers=0
time=2026-09-21T23:59:04.282+09:00 level=INFO msg="spawning new worker" num_old_workers=0
time=2026-09-21T23:59:04.284+09:00 level=INFO msg="starting new worker" pid=29361
time=2026-09-21T23:59:04.284+09:00 level=INFO msg="new worker running, sending signal to old workers" signal=TERM workers=[29033]
time=2026-09-21T23:59:04.284+09:00 level=INFO msg="waiting before killing old workers" delay_seconds=0
time=2026-09-21T23:59:04.284+09:00 level=INFO msg="killing old workers"

[Step 3: 旧プロセスの Graceful Shutdown]
2026/09/21 23:59:04 Received signal: terminated. Initiating graceful shutdown...
2026/09/21 23:59:04 Dewy App stopped gracefully.
time=2026-09-21T23:59:04.285+09:00 level=INFO msg="old worker died" pid=29033 status=0

[Step 4: 新プロセス v0.0.4 がソケットを引き継いで稼働開始]
2026/09/21 23:59:04 Starting Dewy App (Version: v0.0.4)...
2026/09/21 23:59:04 Running under server-starter. Inheriting listening socket.
2026/09/21 23:59:04 Server listening on 0.0.0.0:8080
```

### 動作メカニズム
1. Dewy はポーリングにより S3 上の新バージョン（`v0.0.4`）の存在を検知。
2. 新バイナリ（`dewy-app_darwin_arm64.tar.gz`）をダウンロードし、`.dewy/releases/v0.0.4/` へ展開後、シンボリックリンク `.dewy/current` をアトミックに更新。
3. `server-starter` の親プロセスへ `SIGHUP` を送信。
4. `server-starter` がポート 8080 のリスニングソケットを保持したまま、新プロセス（PID: 29361, v0.0.4）を起動。
5. 新プロセスのヘルスチェック完了後、旧プロセス（PID: 29033, v0.0.3）へ `SIGTERM` を送信。
6. 旧プロセスが安全に既存リクエストの処理を終えて Graceful Shutdown（`status=0`）。
7. **リクエストの取りこぼし（ダウンタイム）はゼロで瞬時に新バージョンへの切り替えが完了！**

---

## 4. `dewyctl` のプロダクト化

一連の検証から得られた知見を結集し、`dewyctl` を独立した汎用 Go CLI ツールとしてリポジトリルートへ昇格しました：
- **`go install github.com/sh0jitmy/dewyctl@latest`** でどこからでも即座にインストール可能。
- ローカル環境でも、CI（GitHub Actions）ランナーでも、全く同一のコマンド（`dewyctl test`, `dewyctl server`, `dewyctl release`）で再現性高く動作。
