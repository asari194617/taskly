locals {
  target_user     = urlescape(getenv("MYSQL_USER"))
  target_password = urlescape(getenv("MYSQL_PASSWORD"))
  target_database = urlescape(getenv("MYSQL_DATABASE"))
}

env "taskly" {
  src     = "file://apps/server/schema.sql"
  url     = "mysql://${local.target_user}:${local.target_password}@${getenv("DB_HOST")}:${getenv("DB_PORT")}/${local.target_database}"
  dev     = getenv("ATLAS_DEV_URL")
  exclude = ["web_push_v_api_ds"]
}
