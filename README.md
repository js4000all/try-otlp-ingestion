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
| `-send-interval` | `0` | OTLP送信リクエスト開始間の最小間隔（例: `100ms`）。`0` は無制限。`-output otlp` のみ対応 |
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

## CollectorからVictoriaMetrics / Grafana Cloud / Mackerelへの投入

デフォルトではComposeでCollectorとVictoriaMetricsを起動し、VictoriaMetricsに送信します。公開ポートはローカルホスト限定です。

```sh
make up
make ps

# GoコンテナからCollectorへ送信（19系列を10秒間隔で5回）
make run-otlp ARGS='-count 5 -interval 10s'

# 登録されたメトリクス名を確認
curl -s 'http://localhost:8428/api/v1/label/__name__/values' | jq .

# S001/A01の系列
curl -G 'http://localhost:8428/api/v1/query' \
  --data-urlencode 'query={site_id="S001",area="A01"}' | jq .

# 温度メトリクス
curl -G 'http://localhost:8428/api/v1/query' \
  --data-urlencode 'query=temperature{site_id="S001",area="A01"}' | jq .

make logs
make down
```

`-output stdout`（既定値）は従来の独自JSON Lines出力です。`-output otlp` は**OTLP/HTTP JSON**に変換して送信します。Resource/Scope/DataPoint属性、GaugeとCumulative Sumを保持します。HTTPエラーが返された場合は生成を停止します。

```sh
# デフォルトはCompose内CollectorのURL
make run ARGS='-output otlp -count 5 -interval 10s'

# Goコンテナ内からアクセスできる別のCollectorへ送信
make run-otlp OTLP_ENDPOINT=http://otel-collector:4318/v1/metrics ARGS='-count 5 -interval 10s'

# ホストから直接実行するときは localhost を指定
./bin/otlpgen -output otlp -otlp-endpoint http://localhost:4318/v1/metrics -count 5
```

`-otlp-endpoint` は**`/v1/metrics`を含む完全なURL**です。Compose内のGoコンテナからは `localhost` ではなく `otel-collector` のサービス名を使用します。

CollectorからVictoriaMetricsへの転送先も変更できます。Compose起動前に `VM_OTLP_METRICS_ENDPOINT` を指定してください。

```sh
VM_OTLP_METRICS_ENDPOINT=http://victoriametrics:8428/opentelemetry/v1/metrics make up
```

単一ノードVictoriaMetricsのOTLP受信URLは `http://victoriametrics:8428/opentelemetry/v1/metrics` です。デフォルトのCollector設定は `collector/local.yaml` にあります。VictoriaMetricsの保存データは `vm-data` ボリュームに保持され、`make down` では消去しません。

### 過去データの流量制御

`-interval` はメトリクスのサンプリング時刻の間隔、`-send-interval` は **OTLPリクエスト開始時刻間の最小間隔** です。後者は `-output otlp` の場合のみ指定でき、既定値の `0` ではレート制限を行いません。送信は順次実行されるため、通信が遅い場合はさらに低速になります。未来時刻のデータは既存どおり該当時刻まで待機します。

```sh
# 2026-11-06 UTCから3日分を、1分間隔のサンプルで生成し、最大10リクエスト/秒で送信
make run-otlp ARGS='-start 2026-11-06 -days 3 -interval 1m -send-interval 100ms'
```

1サンプリング時刻ごとに1リクエストを送るため、3日・1分間隔で4,320リクエストです。100ms指定なら通信時間を除いて約7分12秒です。OTLP受信成功は最終バックエンドへの保存完了を保証しません。Grafana Cloudの送信キューは1 consumer、上限1,000リクエスト、満杯時ブロック、失敗時の指数バックオフ付きリトライを設定しています。メモリキューなのでCollector停止時に未送信データが失われる可能性があります。CollectorのログやGrafana Cloudの検索結果で保存状況を確認してください。

過去時刻のメトリクスはバックエンドのタイムスタンプ受入条件に従います。同じ時系列に新しいデータが既にある場合、古いデータの追加が拒否される場合があります。バックフィルでは時刻順に投入し、検証用の新しい系列を使うことを推奨します。

### Collector送信先の切り替え

Collector設定は `collector/local.yaml`（VictoriaMetrics用）、`collector/grafana-cloud.yaml`（Grafana Cloud用）、`collector/mackerel.yaml`（Mackerel用）から選択できます。Composeが `OTEL_COLLECTOR_CONFIG` を展開します。未指定の場合は `local.yaml` が使われ、`.env` がなくてもローカル構成を起動できます。

```sh
cp .env.example .env
# .envにOTEL_COLLECTOR_CONFIGとGrafana Cloudの認証情報を設定
make up
make run-otlp ARGS='-count 5 -interval 10s'

# .envを変更せずに今回だけGrafana Cloud用設定を使う
OTEL_COLLECTOR_CONFIG=grafana-cloud.yaml make up
```

Grafana Cloudの「Collector setup」で取得したOTLP endpoint（`https://.../otlp`）、instance ID、書き込み権限付きtokenを、それぞれ `GRAFANA_CLOUD_OTLP_ENDPOINT`、`GRAFANA_CLOUD_INSTANCE_ID`、`GRAFANA_CLOUD_API_KEY` に設定してください。Grafana Cloud用の設定はOTLP/HTTPでmetricsを送信します。現在の`otlpgen`はmetricsのみを生成します。

### MackerelへのOTLP投入

Mackerelの**ラベル付きメトリック**を利用できるプランと、Write権限を持つAPIキーが必要です。Mackerel用プロファイルは、ローカルのCollectorではOTLP/HTTPを受信し、Mackerelの `otlp.mackerelio.com:4317` に **TLS付きOTLP/gRPC** で転送します。Grafana Cloudとは接続方式が異なりますが、`otlpgen` の変更は不要です。

```sh
cp .env.example .env
# .env を編集:
# OTEL_COLLECTOR_CONFIG=mackerel.yaml
# MACKEREL_APIKEY=<Write権限を持つAPIキー>

make up
make run-otlp ARGS='-count 1'
docker compose logs --tail=100 otel-collector
```

`.env` を変更せずにプロファイルだけ一時的に選ぶ場合は、`OTEL_COLLECTOR_CONFIG=mackerel.yaml make up` も利用できます（APIキーは `.env` に設定してください）。変更後に古いCollectorが動作している場合は `docker compose up -d --force-recreate otel-collector` で再作成します。

Mackerel用の送信キューは1 consumer、容量1,000、満杯時ブロック、リトライは初回5秒・最大30秒・継続10分です。Collectorが受信に成功してもMackerelでの保存を保証しません。キューはメモリ上にあり、Collector停止時には未送信分が失われ得ます。送信後はMackerelのラベル付きメトリック画面とCollectorのログを確認してください。

モデル互換性の検証では、`temperature`、`vibration`、`運転中`、`plc_reader.read_errors` の名前、Resource属性（`site_id`、`area` など）、DataPoint属性（`sensor_id`、`position` など）、GaugeとCumulative Sumの扱いを比較します。変換結果は実際の検索結果で確認し、あらかじめ同名で保存されることは仮定しません。

Composeの `.env` は設定選択に使われ、`env_file` によりCollectorコンテナにも渡されます。`.env` はGit管理対象外です。**認証情報をコミットしないでください。** 設定を切り替えた場合は、Collectorの再作成を確認してください（必要なら `docker compose up -d --force-recreate otel-collector`）。

Grafana Cloud向け設定を選択しても、現状の `make up` はVictoriaMetricsコンテナも起動しますが、Grafana Cloud向けpipelineはVictoriaMetricsへ送信しません。ローカル設定とクラウド設定は独立しています。

ローカル検証専用で、認証やTLSは設定していません。公開ネットワークにポートを開放しないでください。
