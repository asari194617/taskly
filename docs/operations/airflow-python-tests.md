# Airflow Python テスト

`infra/airflow/tests` は休日カレンダー、日直通知対象の選択、DAG 全体の読み込みを検証します。PR の `compose-build-ci` は Airflow イメージのビルド後、同じイメージ内でこれらのテストを実行します。Airflow またはその DAG、テストを変更した PR ではイメージが選択されます。

ローカルで CI と同じ依存関係を使う場合は、リポジトリのルートで次を実行します。Docker Engine が必要です。

```sh
docker build -f infra/airflow/Dockerfile -t taskly-airflow-test .
docker run --rm \
  --volume "$PWD/infra/airflow:/opt/airflow/ci:ro" \
  --entrypoint python taskly-airflow-test \
  -m unittest discover -s /opt/airflow/ci/tests -v
```

イメージには Airflow 3.3.0 / Python 3.14 と、Airflow の constraints に沿ってインストールされた `icalendar`、`python-dateutil` などが含まれます。`test_dag_imports.py` は全 DAG を `DagBag` で読み込み、import エラーと DAG ID の欠落を失敗として扱います。新しい DAG を追加したら期待する ID も更新してください。
