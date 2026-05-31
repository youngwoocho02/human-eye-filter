# Human Eye Filter

[English](README.md) | [Korean](README.ko.md) | [Japanese](README.ja.md)

> AI に渡る前にコマンド出力を読みやすいサイズへ畳み込むパイプフィルター。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**モード選択なし。ツール別アダプターなし。`| hef` を付けるだけです。**

## なぜ必要か

人間は数千文字の出力から必要な行だけを素早く拾えます。しかし AI モデルには、繰り返しのパス、重複行、長い ID、大きなダンプもすべてトークンとして入ります。`hef` は人間が読み飛ばす繰り返しを畳み込み、読みやすさを保ったままトークンの無駄を減らします。

`hef` はコマンドランナーではありません。元のコマンドはそのままで、stdin の出力だけを減らして stdout に出します。

## インストール

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.ps1 | iex
```

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.sh | sh
```

## クイックスタート

```sh
some-command | hef
rg -n --glob '*.cs' 'ContentAvailability' Assets | hef
```

## どう縮むか

### 検索結果

長い `path:line:text` 出力はファイルごとにまとまります。

```text
C:/Work/Project/Assets/Foo.cs:10: TODO: split setup
C:/Work/Project/Assets/Foo.cs:48: FIXME: stale state
C:/Work/Project/Assets/Bar.cs:31: TODO: wire label
```

```text
$root=C:/Work/Project/Assets
$root/Foo.cs (2)
  10: TODO: split setup
  48: FIXME: stale state
$root/Bar.cs (1)
  31: TODO: wire label
```

### ファイル一覧

大きなファイル一覧はフルパスを繰り返さず、ディレクトリ構造を残します。

```text
C:/Work/Project/Assets/CookStation/Blender/A.cs
C:/Work/Project/Assets/CookStation/Blender/B.cs
C:/Work/Project/Assets/CookStation/Brewer/C.cs
```

```text
$root=C:/Work/Project/Assets/CookStation
$root/Blender/ (2)
  A.cs
  B.cs
$root/Brewer/ (1)
  C.cs
```

### 繰り返し

同じ行、連番、繰り返される長いトークンをその場で畳みます。

```text
loading
loading
loading
file_001.tmp
file_002.tmp
file_003.tmp
created 550e8400-e29b-41d4-a716-446655440000
updated 550e8400-e29b-41d4-a716-446655440000
```

```text
$t1=550e8400-e29b-41d4-a716-446655440000
loading (x3)
file_001..003.tmp (3 lines)
created $t1
updated $t1
```

出力が大きすぎる場合は、先頭、重要行、末尾だけを残し、省略量を表示します。

## アップデート

```sh
hef update
hef update --check
```

## エージェント設定

エージェントは通常どおりシェルコマンドを実行します。フックやプラグインはコマンドの後ろに `| hef` を付けるだけです。

```sh
hef setup --agent all
```

対応エージェント:

- `claude`: Claude Code `PreToolUse` フックスクリプトを書き込みます。
- `codex`: OpenAI Codex `PreToolUse` フックスクリプトを書き込みます。
- `opencode`: OpenCode `tool.execute.before` プラグインを書き込みます。

削除:

```sh
hef setup --agent all --remove
```

対象はシェルコマンドツールだけです。直接のファイル読み取り、ブラウザ、画像、その他の非シェルツールの出力は `hef` を通りません。

## オプション

| Flag                | Default   | Description |
| ------------------- | --------- | ----------- |
| `-max-lines`        | `160`     | 最大出力行数 |
| `-max-chars`        | `12000`   | 最大出力文字数 |
| `-max-input-bytes`  | `4194304` | 圧縮前に読む最大バイト数 |
| `-focus`            | _(none)_  | 優先して残すキーワード |
| `-raw-on-fail`      | `true`    | 縮小に失敗した場合に原文を出力 |
| `-version`          | `false`   | バージョンを表示 |

ほとんどの実行でオプションは不要です。

## 開発

```sh
gofmt -w .
go test ./...
go build ./...
```

## ライセンス

MIT。詳細は [LICENSE](LICENSE) を参照してください。
