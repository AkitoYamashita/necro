# necro

necro（Necromancer）は、複数のAWS SSOプロファイルを横断してAWS CLI操作を実行するための小さなCLIツールです。

1つのtask定義から、複数アカウントへ同一操作を安全に実行できます。

![alt text](necro.png)

---

## 🎯 目的（コンセプト）

- ~/.aws/config に定義された複数のSSOプロファイルへ一括実行
- デフォルトリージョンは ap-northeast-1
- idempotent（冪等）志向を意識した運用
- 単一バイナリ配布でどこでも実行可能（AWS CLIが必要）
- JSON出力を前提にした安全な結果評価
- capture / if / foreach による条件分岐・展開実行

---

## 🚀 インストール

### Windows

GitHub Releases から

    necro_windows_amd64.exe

をダウンロードして実行。

### Mac（arm64 / Apple Silicon）

    necro_darwin_arm64

をダウンロード後：

    xattr -d com.apple.quarantine necro_darwin_arm64
    chmod +x necro_darwin_arm64

---

## 🧪 使い方

### 1. AWS SSO設定

サンプルを参考に：

    conf/sample_config

をもとに

    ~/.aws/config

を作成。

SSOログインを実施(認証情報のキャッシュで動作するため)：

    aws sso login --profile SAMPLE_PROFILE

---

### 2. task定義を作成

サンプル：

    conf/sample_task.yml

コピーして編集：

    cp conf/sample_task.yml conf/task.yml

編集：

    conf/task.yml

---

### 3. 実行

ビルド情報確認：

    necro version

ドライラン（実行せず確認）：

    necro conf/task.yml --dry-run

実行：

    necro conf/task.yml

---

## 🧠 taskファイル構造

### 基本構造

    version: 1

    defaults:
      region: ap-northeast-1

    targets:
      profiles: []
      exclude: []

    vars:
      defaults:
        KEY: value
      profiles:
        PROFILE_NAME:
          KEY: value

    cmd:
      - name: example
        run: ["s3api","list-buckets"]

---

## 🔧 主な機能

### ✔ capture

AWS CLIのJSON出力から値を変数として取得

    capture:
      CHANGE_SET_ID: "Id"

---

### ✔ if 分岐

JMESPath式で条件分岐

    if:
      expr: "Status"
      op: "eq"
      value: "FAILED"
    ok:
      - name: delete
        run: [...]
    ng:
      - name: execute
        run: [...]

対応演算子：

- eq
- ne
- contains
- exists
- in

---

### ✔ foreach

配列を展開して実行

    foreach:
      var: CHANGE_SET_NAMES
      as: CHANGE_SET_NAME

---

## 📋 実行ログ

- log/<RUN_ID>.txt に自動出力
- STSチェック結果表示
- 実行時間計測
- 成功 / 失敗を明示表示

---

## 🧹 ログ管理（Makefile例）

    clean_logs: ## keep latest 5 log/*.txt and remove older ones
        ls -1t log/*.txt 2>/dev/null | tail -n +6 | xargs -r rm -f

---

## 🛠 要件

- AWS CLI v2
- ~/.aws/config に SSO プロファイルが存在
- SSOセッションが有効

未ログインの場合：

    aws sso login --profile <name>

---

## 💀 Philosophy

necroは

- Shell地獄に戻らない
- jqに依存しない
- できるだけシンプルに
- でも十分に強力に

を目指したツールです。
