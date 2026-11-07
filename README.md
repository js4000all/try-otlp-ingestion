# try-otlp-ingestion

OTLPデータの保存・検索を比較するためのダミーデータ生成ツールです。Goの実行環境はDocker Composeで提供し、ホストにGoをインストールする必要はありません。

## 実行

Docker Engine / Docker Compose v2 とMakeが必要です。

```sh
# 2026-11-01 UTC 00:00から2日分を1時間間隔で生成
make run ARGS='-start 2026-11-01 -days 2 -interval 1h' > samples.jsonl

# JSTで開始時刻を指定（タイムゾーン付きRFC3339）
make run ARGS='-start 2026-11-07T12:00:00+09:00 -days 1 -interval 1m -seed 42'

make test
make check
```

| 引数 | 既定値 | 意味 |
|---|---|---|
| `-start` | 当日のUTC日付 | `YYYY-MM-DD`（UTC 00:00）またはRFC3339日時 |
| `-days` | `1` | 開始日からの日数（開始を含み終了を含まない） |
| `-interval` | `1m` | サンプリング間隔（`10s`, `5m`, `1h` 等） |
| `-seed` | `1` | 乱数シード |
| `-config` | `config` | 設定ファイルのディレクトリ |

**過去時刻は即時に生成し、未来時刻はその対象時刻まで待機**します。Ctrl+Cで停止できます。長期間・短間隔では出力量が増える点に注意してください。

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
