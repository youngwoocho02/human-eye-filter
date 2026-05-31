# Human Eye Filter

[English](README.md) | [Korean](README.ko.md) | [Japanese](README.ja.md)

> AI に渡る前にコマンド出力を読みやすいサイズへ畳み込むパイプフィルター。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**モード選択なし。ツール別アダプターなし。`| hef` を付けるだけです。**

## なぜ必要か

人間は数千文字の出力から必要な行だけを素早く拾えます。しかし AI モデルには、繰り返しのパス、重複行、長い ID、大きなダンプもすべてトークンとして入ります。`hef` は人間が読み飛ばす繰り返しを畳み込み、読みやすさを保ったままトークンの無駄を減らします。

`hef` はコマンドランナーではありません。元のコマンドはそのままで、stdin の出力だけを減らして stdout に出します。

## RTK との違い

RTK は command proxy です。エージェントのフックが `git status` のようなコマンドを `rtk git status` に書き換え、RTK がそのコマンドをコマンド別フィルターへ送ります。

`hef` は単純な出力フィルターです。コマンドを置き換えず、ツール別 command wrapper も不要で、ユーザーが mode を選ぶ必要もありません。元のコマンドはそのまま実行され、`hef` は stdout の繰り返しパターンだけを畳みます。

```sh
# RTK style
agent command -> hook rewrite -> rtk <command> -> command-specific filter

# HEF style
agent command -> original command runs -> stdout | hef -> pattern filters
```
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

エージェントのフックを一度登録すれば完了です。その後、エージェントは通常どおりシェルコマンドを実行し、`hef` が自動で付きます。

```sh
hef setup --agent all
```
自分でフィルターしたい場合は、手動のパイプもそのまま使えます。

```sh
some-command | hef
```
## どう縮むか

`hef` は 7 つの折りたたみパスを実行し、各パスは結果が実際に短くなる場合のみ適用され、最後に 1 行のサマリーフッターを付加します。以下の例はすべて実際の `hef` 出力です。

### 1. 繰り返し行

連続する同一行は回数付きの 1 行に折りたたまれます。

```text
compiling module foo
warning: unused variable x
warning: unused variable x
warning: unused variable x
warning: unused variable x
done
```

```text
compiling module foo
warning: unused variable x (x4)
done

--- hef summary ---
filters=RepeatedLine lines=6->3 chars=133->57
```

### 2. 連番の範囲

連番が付いたファイルや行の連なりは 1 つの範囲に折りたたまれます。

```text
created file_001.tmp
created file_002.tmp
created file_003.tmp
created file_004.tmp
created file_005.tmp
created file_006.tmp
```

```text
created file_001..006.tmp (6 lines)

--- hef summary ---
filters=SequentialRange lines=6->1 chars=125->35
```

### 3. Grep 形式の結果をファイルごとにグループ化

`path:line:text` 形式のヒットはファイルごとにまとめられ、共有するワークスペースのパスは `$root` エイリアスに引き上げられ、長いヒット一覧は一定数で打ち切られます。

```text
C:/WorkSpace/proj/src/app/main.go:10:func main() {
C:/WorkSpace/proj/src/app/main.go:25:return
C:/WorkSpace/proj/src/app/main.go:41:log.Fatal(err)
C:/WorkSpace/proj/src/app/main.go:58:os.Exit(1)
C:/WorkSpace/proj/src/app/main.go:77:}
C:/WorkSpace/proj/src/db/store.go:12:type Store struct {
C:/WorkSpace/proj/src/db/store.go:30:func Open() {
```

```text
$root=C:/WorkSpace/proj/src
$root/app/main.go (5)
  10: func main() {
  25: return
  41: log.Fatal(err)
  58: os.Exit(1)
  ... <1 more>
$root/db/store.go (2)
  12: type Store struct {
  30: func Open() {

--- hef summary ---
filters=PathLineGroup lines=7->10 chars=341->203
```

### 4. ファイル一覧をディレクトリごとにグループ化

フラットなファイル一覧はディレクトリごとにまとめられ、同じく `$root` エイリアスを共有します。

```text
C:/WorkSpace/proj/internal/core/router.go
C:/WorkSpace/proj/internal/core/handler.go
C:/WorkSpace/proj/internal/core/config.go
C:/WorkSpace/proj/internal/core/logger.go
C:/WorkSpace/proj/internal/core/server.go
C:/WorkSpace/proj/internal/core/client.go
C:/WorkSpace/proj/internal/api/routes.go
C:/WorkSpace/proj/internal/api/middleware.go
```

```text
$root=C:/WorkSpace/proj/internal
$root/api/ (2)
  middleware.go
  routes.go
$root/core/ (6)
  client.go
  config.go
  handler.go
  logger.go
  router.go
  server.go

--- hef summary ---
filters=DirectoryGroup lines=8->11 chars=338->164
```

### 5. 共通プレフィックス

多くの行が長い共通プレフィックス（20 文字以上）を共有する場合、それを `$prefix` エイリアスに引き上げます。

```text
/var/lib/myapp/cache/session-alpha.bin
/var/lib/myapp/cache/session-beta.log
/var/lib/myapp/cache/session-gamma.tmp
/var/lib/myapp/cache/session-delta.dat
```

```text
$prefix1=/var/lib/myapp/cache/session-
$prefix1alpha.bin
$prefix1beta.log
$prefix1gamma.tmp
$prefix1delta.dat

--- hef summary ---
filters=CommonPrefix lines=4->5 chars=154->109
```

### 6. 辞書トークン

長く繰り返される文字列（24 文字以上: GUID、URL、パス）は一度だけ定義し、すべての箇所でエイリアスに置き換えます。

```text
ref 550e8400-e29b-41d4-a716-446655440000 start
mid 550e8400-e29b-41d4-a716-446655440000 more
end 550e8400-e29b-41d4-a716-446655440000 tail
```

```text
$t1=550e8400-e29b-41d4-a716-446655440000
ref $t1 start
mid $t1 more
end $t1 tail

--- hef summary ---
filters=DictionaryToken lines=3->4 chars=138->80
```

### 7. 上限付きサンプリング

出力が行の上限（`-max-lines`）を超えると、`hef` は先頭のサンプルと末尾のサンプルを残し、中間を捨てつつ、失われては困る重要な行（`error`、`fatal`、`panic` など）は救い出します。8 行目がエラーである次の 15 行に `hef -max-lines 10` を適用すると:

```text
starting build pipeline
loading dependency graph
resolving module versions
compiling core package
linking shared objects
generating documentation
optimizing image assets
ERROR undefined symbol in audio engine
running unit tests
measuring code coverage
packaging release archive
signing platform binaries
uploading to artifact store
notifying release channel
cleaning temporary workspace
```

```text
## head
starting build pipeline
loading dependency graph
resolving module versions
compiling core package
linking shared objects
generating documentation
## important
ERROR undefined symbol in audio engine
## tail
measuring code coverage
packaging release archive
signing platform binaries
uploading to artifact store
notifying release channel
cleaning temporary workspace
... <omitted 3 lines>

--- hef summary ---
filters=BoundedSample lines=15->17 chars=386->394
```

`ERROR` 行は `## important` ブロックに救い出されるため、失敗が静かに消えることはありません。

いずれのパスが実行される前に、入力は正規化されます。ANSI カラーエスケープ、NUL バイト、CR/CRLF 改行が除去されます。また毎回の実行で、どのフィルタが動作し行数・文字数がどれだけ削減されたかを示す `--- hef summary ---` フッターが出力末尾に付加されます。
## アップデート

```sh
hef update
hef update --check
```
## エージェント設定

`hef setup` は対応エージェントのフックやプラグインファイルを書き込みます。

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
