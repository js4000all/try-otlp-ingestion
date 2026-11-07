# try-otlp-ingestion

OTLPで計測データを投入し、各データストアでの保存・検索方法を比較するための実験用リポジトリです。

この段階では **Goの開発環境のみ** を用意しています。ダミーデータ生成、OTLP送信、OpenTelemetry Collectorやデータストアの構成は今後追加します。

## 前提

- Docker EngineとDocker Compose v2（`docker compose`）
- Make

ホスト側へのGoコンパイラのインストールは不要です。

## 使い方

```sh
make help    # コマンド一覧
make run     # 動作確認用プログラムを実行
make build   # bin/otlpgen にLinux向けバイナリを生成
make test    # go test ./...
make vet     # go vet ./...
make fmt     # go fmt ./...
make tidy    # go mod tidy
make check   # fmt-check、test、vet、build
make shell   # Goコンテナ内でシェルを起動
make clean   # ローカルの生成物とキャッシュを削除
```

GoコマンドはすべてComposeの `go` サービス内で実行します。ソースコード、`bin/` と `.cache/` はローカルの作業ディレクトリに置かれますが、`bin/` と `.cache/` はGit管理対象外です。MakefileからホストのUID/GIDを渡すので、生成物の所有者は通常ホストのユーザーと一致します。

`make build` はLinux用バイナリを生成します。macOS/Windowsホスト上での動作確認には `make run` を利用してください。

## 構成

```text
.
├── .github/workflows/ci.yml  # PR時のチェック
├── cmd/otlpgen/main.go        # 起動確認用の最小プログラム
├── compose.yaml               # Go開発用コンテナ
├── go.mod
└── Makefile
```

## 今後の作業

テストデータのResource / Scope / Metric / DataPointの設計を確定してから、OTLP Metrics生成・送信処理とCollectorを追加します。
