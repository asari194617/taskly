#!/usr/bin/env bash
# Run after the new Airflow image is pulled and before any Airflow service starts.
set -euo pipefail

config=$(docker compose config --format json)
MYSQL_DATABASE=$(jq -er '.services.db.environment.MYSQL_DATABASE' <<<"$config")
MYSQL_USER=$(jq -er '.services.db.environment.MYSQL_USER' <<<"$config")
AIRFLOW_META_DATABASE=$(jq -er '.services["airflow-webserver"].environment.AIRFLOW_META_DATABASE' <<<"$config")
AIRFLOW_META_USER=$(jq -er '.services["airflow-webserver"].environment.AIRFLOW_META_USER' <<<"$config")
AIRFLOW_META_PASSWORD=$(jq -er '.services["airflow-webserver"].environment.AIRFLOW_META_PASSWORD' <<<"$config")
unset config

identifier='^[a-zA-Z][a-zA-Z0-9_]*$'
for value in "$MYSQL_DATABASE" "$MYSQL_USER" "$AIRFLOW_META_DATABASE" "$AIRFLOW_META_USER"; do
  [[ $value =~ $identifier ]] || { echo 'Invalid MySQL identifier' >&2; exit 1; }
done
[[ $MYSQL_DATABASE != "$AIRFLOW_META_DATABASE" && $MYSQL_USER != "$AIRFLOW_META_USER" && $MYSQL_USER != root ]] || {
  echo 'Application and Airflow database names and users must differ' >&2; exit 1;
}
# The password is interpolated into both SQL and the SQLAlchemy URL.
[[ $AIRFLOW_META_PASSWORD =~ ^[a-zA-Z0-9_-]+$ ]] || {
  echo 'AIRFLOW_META_PASSWORD must contain only letters, digits, underscores or hyphens' >&2; exit 1;
}

mysql_root() {
  docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --batch --skip-column-names "$@"' sh "$@"
}
mysql_root -e 'SELECT 1' >/dev/null

schema_exists=$(mysql_root -e "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = '$AIRFLOW_META_DATABASE'")
if [[ $schema_exists == 1 ]]; then
  existing_tables=$(mysql_root -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$AIRFLOW_META_DATABASE'")
else
  existing_tables=0
fi
old_dag_runs=$(mysql_root -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$MYSQL_DATABASE' AND table_name = 'dag_run'")
if [[ $existing_tables != 0 && $old_dag_runs != 0 ]]; then
  echo 'Both databases contain Airflow tables; inspect them before retrying' >&2
  exit 1
fi

mysql_root <<SQL
CREATE DATABASE IF NOT EXISTS \`$AIRFLOW_META_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$AIRFLOW_META_USER'@'%' IDENTIFIED BY '$AIRFLOW_META_PASSWORD';
ALTER USER '$AIRFLOW_META_USER'@'%' IDENTIFIED BY '$AIRFLOW_META_PASSWORD';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM '$AIRFLOW_META_USER'@'%';
GRANT ALL PRIVILEGES ON \`$AIRFLOW_META_DATABASE\`.* TO '$AIRFLOW_META_USER'@'%';
SQL

restrict_app_grants() {
  mysql_root <<SQL
REVOKE ALL PRIVILEGES, GRANT OPTION FROM '$MYSQL_USER'@'%';
GRANT ALL PRIVILEGES ON \`$MYSQL_DATABASE\`.* TO '$MYSQL_USER'@'%';
SQL
}

verify_isolation() {
  # Temporary tables disappear with the connection even if an unexpected
  # grant allows the operation. Successful execution is a security failure.
  docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -h 127.0.0.1 -u"$MYSQL_USER" "$MYSQL_DATABASE" -e "SELECT 1"' >/dev/null
  docker compose exec -T db sh -c 'MYSQL_PWD="$AIRFLOW_META_PASSWORD" exec mysql -h 127.0.0.1 -u"$AIRFLOW_META_USER" "$AIRFLOW_META_DATABASE" -e "SELECT 1"' >/dev/null
  if docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -h 127.0.0.1 -u"$MYSQL_USER" -e "CREATE TEMPORARY TABLE \`$AIRFLOW_META_DATABASE\`.taskly_privilege_probe (id INT)"' >/dev/null 2>&1; then
    echo 'Application account can write to the Airflow metadata schema' >&2
    exit 1
  fi
  if docker compose exec -T db sh -c 'MYSQL_PWD="$AIRFLOW_META_PASSWORD" exec mysql -h 127.0.0.1 -u"$AIRFLOW_META_USER" -e "CREATE TEMPORARY TABLE \`$MYSQL_DATABASE\`.taskly_privilege_probe (id INT)"' >/dev/null 2>&1; then
    echo 'Airflow metadata account can write to the application schema' >&2
    exit 1
  fi
}

if [[ $old_dag_runs == 0 ]]; then
  # Fresh install, or a completed migration being replayed by deployment.
  docker compose run --rm --no-deps --entrypoint airflow airflow-webserver db migrate </dev/null
  restrict_app_grants
  verify_isolation
  echo 'Airflow metadata database is ready'
  exit 0
fi

# The new image defines the exact metadata table set. Create a disposable
# schema, intersect its tables with the legacy mixed schema, then move that set
# atomically. Moving keeps DAG schedules, runs, task history and FAB users.
docker compose run --rm --no-deps --entrypoint airflow airflow-webserver db migrate </dev/null
mapfile -t tables < <(mysql_root -e "SELECT n.table_name FROM information_schema.tables n JOIN information_schema.tables o ON o.table_schema = '$MYSQL_DATABASE' AND o.table_name = n.table_name WHERE n.table_schema = '$AIRFLOW_META_DATABASE' AND n.table_type = 'BASE TABLE' ORDER BY n.table_name")
(( ${#tables[@]} > 0 )) || { echo 'No legacy Airflow tables were found' >&2; exit 1; }

declare -A found=()
for table in "${tables[@]}"; do
  [[ $table =~ $identifier ]] || { echo 'Unexpected Airflow table name' >&2; exit 1; }
  found[$table]=1
done
for required in alembic_version dag dag_run task_instance; do
  [[ ${found[$required]+yes} ]] || { echo "Missing legacy Airflow table: $required" >&2; exit 1; }
done
for app_table in users subjects tasks sessions migrations logs api_keys; do
  [[ ! ${found[$app_table]+yes} ]] || { echo "Application table collides with Airflow: $app_table" >&2; exit 1; }
done

before_runs=$(mysql_root -e "SELECT COUNT(*) FROM \`$MYSQL_DATABASE\`.dag_run")
before_tasks=$(mysql_root -e "SELECT COUNT(*) FROM \`$MYSQL_DATABASE\`.task_instance")
# The reference schema contains only newly initialized metadata. The old data
# stays in the application schema until this point, so a failed reference
# migration cannot affect it.
mysql_root -e "DROP DATABASE \`$AIRFLOW_META_DATABASE\`; CREATE DATABASE \`$AIRFLOW_META_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"

renames=()
for table in "${tables[@]}"; do
  renames+=("\`$MYSQL_DATABASE\`.\`$table\` TO \`$AIRFLOW_META_DATABASE\`.\`$table\`")
done
sql=$(IFS=,; echo "RENAME TABLE ${renames[*]}")
mysql_root -e "$sql"

# New Airflow versions can add tables that were absent from the old schema.
docker compose run --rm --no-deps --entrypoint airflow airflow-webserver db migrate </dev/null
after_runs=$(mysql_root -e "SELECT COUNT(*) FROM \`$AIRFLOW_META_DATABASE\`.dag_run")
after_tasks=$(mysql_root -e "SELECT COUNT(*) FROM \`$AIRFLOW_META_DATABASE\`.task_instance")
[[ $before_runs == "$after_runs" && $before_tasks == "$after_tasks" ]] || {
  echo 'Airflow history counts changed during migration' >&2; exit 1;
}
restrict_app_grants
verify_isolation
echo "Moved ${#tables[@]} Airflow tables; DAG runs: $after_runs, task instances: $after_tasks"
