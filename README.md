# necro

necro（Necromancer）は、複数のAWS SSOプロファイルを横断してAWS CLI操作を実行するための小さなCLIツールです。

1つのtask定義から、複数アカウントへ同一操作を安全に実行できます。

![alt text](necro.png)

---

## 🎯 コンセプト（v2）

- ~/.aws/config の複数SSOプロファイルへ一括実行
- デフォルトリージョン ap-northeast-1
- Go template + sprig による変数解決
- 変数は収束するまで反復評価（デフォルト10回）
- aws / sh 両方テンプレート対象
- JSON前提の安全な条件分岐
- capture / if / foreach による展開実行
- 単一バイナリ配布（AWS CLI v2 が必要）

---

## 🚀 インストール

### Windows

    necro_windows_amd64.exe

をGitHub Releasesからダウンロード。

---

### Mac（arm64）

    necro_darwin_arm64

ダウンロード後：

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

### 2. sample_task1_startup.yml（プロファイル自動生成）

    conf/sample_task1_startup.yml

は、AWS Organizationsのアカウント一覧から

- ~/.aws/config 用 profileブロック
- necro用 vars.profiles YAML

を生成するサンプルです。

これにより、一部のAWS SSOプロファイル定義を自動生成できます。

---

### 3. task定義作成

    cp conf/sample_task.yml conf/task.yml

編集：

    conf/task.yml

---

### 4. 実行

    necro version

ドライラン（実行せず確認）：

    necro conf/task.yml --dry-run

実行：

    necro conf/task.yml

---

## 🧠 taskファイル構造

    version: 1

    defaults:
      region: ap-northeast-1

    targets:
      profiles: []
      exclude: []

    vars:
      template-resolve-limit: 10

      defaults:
        KEY: value

      profiles:
        PROFILE_NAME:
          KEY: value

    cmd:
      - name: example
        aws: ["s3api","list-buckets"]

---

## 🧩 テンプレート仕様

necroは Go template + sprig を使用します。

対象：

- vars.defaults
- vars.profiles
- aws引数
- sh
- out
- capture後変数

例：PROFILEからSYSTEM/ENV自動導出

    SYSTEM: '{{ (splitList "_" .PROFILE | first | lower) }}'
    ENV: '{{ (splitList "_" .PROFILE | last  | lower) }}'

例：相互参照

    BUCKET_NAME: 's3-{{ .SYSTEM }}-{{ .ENV }}-artifact'
    TEMPLATE_URL: 'https://{{ .BUCKET_NAME }}.s3.{{ .REGION }}.amazonaws.com/template.yml'

---

## 🔧 主な機能

### ✔ capture

    capture:
      CHANGE_SET_ID: "Id"

capture後もテンプレート変数は再解決されます。

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

- log/<RUN_ID>.txt に自動保存
- STS事前チェック
- 実行時間表示
- 成功 / 失敗明示
- RUN_ID は実行単位で自動生成

## 🛠 要件

- AWS CLI v2
- ~/.aws/config に SSO プロファイル
- SSOログイン済み

未ログイン時：

    aws sso login --profile <name>