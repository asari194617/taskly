# Task search with OpenSearch

Task lists use OpenSearch for nonempty title and description searches when `OPENSEARCH_URL` is set and the initial index is ready. REST and GraphQL use the same path. Empty queries and MCP owner lists continue to use MySQL. The API falls back to MySQL if OpenSearch cannot be reached. The MySQL `tasks` table remains the source of truth; matching IDs are loaded again from MySQL with deadline, visibility, and text filters before they are returned.

## Local setup

The OpenSearch Compose service is behind the `search` profile. It has a persistent volume and no published host port. Set these values in `.env`:

```dotenv
OPENSEARCH_URL=http://opensearch:9200
OPENSEARCH_USERNAME=
OPENSEARCH_PASSWORD=
```

Then start the stack with `COMPOSE_PROFILES=search docker compose -f infra/docker-compose.yml up -d --build`. Docker hosts need `vm.max_map_count` of at least 262144. The default profile leaves OpenSearch stopped and uses MySQL search.

## Production rollout

Provision enough memory for OpenSearch in addition to the existing services, or use a separate OpenSearch endpoint. The current small production host should **not** enable the `search` profile until its memory capacity is increased. The bundled service uses a 512 MiB JVM heap and needs memory beyond that heap for the OS, native allocations, and file cache.

For the bundled service, set `OPENSEARCH_URL=http://opensearch:9200` in the production environment and include `search` in `PROD_COMPOSE_PROFILES`. The deployment starts the service before the API. For an external endpoint, set `OPENSEARCH_URL` to its HTTPS URL and, when required, set `OPENSEARCH_USERNAME` and `OPENSEARCH_PASSWORD`. Do not put credentials in the URL. The API uses a three second HTTP timeout.

Migration `0026_task_search_outbox.sql` adds MySQL triggers that record inserts, updates, and deletes from both API requests and batch jobs. The API scans existing tasks on startup, then processes the outbox continuously. Search uses MySQL until that first scan and outbox replay finish. New edits normally become searchable after the next one second polling cycle and OpenSearch refresh. If indexing fails, the outbox entry remains for retry. A restart repeats the full scan, so a lost index can be rebuilt without a manual command.

To verify after activation, check `docker compose logs server opensearch` for index errors, create or edit a task, and search its title through `/tasks?q=...`. Check a second search after approximately two seconds. Confirm a hidden task is absent for an ordinary user. To roll back indexed search, clear `OPENSEARCH_URL` and redeploy the API. Preserve the OpenSearch volume if you intend to reuse the index.
