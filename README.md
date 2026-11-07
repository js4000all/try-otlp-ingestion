# try-otlp-ingestion

OTLPデータの保存・検索を比較するためのダミーデータ生成ツールです。Goの実行環境はDocker Composeで提供し、ホストにGoをインストールする必要はありません。

## 実行

Docker Engine / Docker Compose v2 とMakeが必要です。

```sh
# 2026-11-01 UTC 00:00から2日分を1時間間隔で生成
make run ARGS='-start 2026-11-01 -days 2 -interval 1h' > samples.jsonl

# JSTで開始時刻を指定（タイムゾーン付きRFC3339）
make run ARGS='-start 2026-11-07T12:00:00+09:00 -days 1 -interval 1m -seed 42'

# 現在時刻を始点として5回生成（初回即時、10秒間隔で計4回待機）
make run ARGS='-count 5 -interval 10s' | jq -c '.resourceMetrics | length'

make test
make check
```

| 引数 | 既定値 | 意味 |
|---|---|---|
| `-start` | 当日のUTC日付 | 期間指定モードの開始日時。`YYYY-MM-DD`（UTC 00:00）またはRFC3339日時 |
| `-days` | `1` | 期間指定モードの日数（開始を含み終了を含まない） |
| `-count` | 指定なし | 現在時刻を始点に指定回数生成する回数指定モード（1以上） |
| `-interval` | `1m` | サンプリング間隔（`10s`, `5m`, `1h` 等） |
| `-seed` | `1` | 乱数シード |
| `-config` | `config` | 設定ファイルのディレクトリ |

**期間指定モード:** 過去時刻は即時に生成し、未来時刻はその対象時刻まで待機します。**回数指定モード:** `-count N` で実行時刻から計N回生成します。初回は即時、インターバルはN−1回です（`-count 1` は待機なし）。`-count` は `-start` / `-days` の明示指定と併用できません。時刻は開始時刻とインターバルから固定して計算するため、出力処理が遅れても時刻のずれが累積しません。Ctrl+Cで停止できます。長期間・短間隔では出力量が増える点に注意してください。

出力は1サンプリング時刻につき1行のJSON Lines（標準出力）です。Resource / Scope / Metric / DataPointを階層化した**OTLP風の独自JSON**であり、公式のOTLP/JSON wire formatではありません。Collectorへの投入形式への変換は今後実装します。

## 設定

| ファイル | 定義 |
|---|---|
| `config/target-metric.yaml` | ターゲット種別のMetricと値の生成パターン、運用Metric |
| `config/site-area-target.yaml` | サイト、区画、ターゲット、センサー |
| `config/plc-target.yaml` | PLC Reader、PLC、ターゲットの対応 |

現在は2サイト・4モーター・3PLCです。温度（軸受: sine、巻線: random walk）、振動（spike）、運転状態（toggle）、PLC読み取りエラー累積（counter）の4 Metricを生成します。サイトIoTと運用監視のドメインはResource属性 `monitoring.domain` で区別します。

## 開発コマンド

```sh
make help
make run ARGS='-start 2026-11-01 -days 1 -interval 1h'
make build
make test
make vet
make fmt
make tidy
make check
make shell
make clean
```

## 開発・push前の必須チェック

コードを変更した場合、**push前に** 次の順序で実行してください。すべてDocker Compose上のGo環境を利用します。

```sh
make tidy
make fmt
make check
```

`make tidy` / `make fmt` により変更されたファイルを確認し、**整形結果も含めてコミットしてからpush**してください。チェックに失敗した場合は修正して再実行します。Goソースは読みやすさを優先し、複数の宣言・条件分岐・ループ・エラー処理を不必要に1行へ圧縮しません。フォーマッターだけで可読性が保証されるわけではないため、差分を目視でも確認してください。

AIによる変更も含めた詳細な規約は [AGENTS.md](AGENTS.md) を参照してください。
