# Production network ports

Taskly publishes only the host ports required by the host Nginx reverse proxy
and host-local monitoring. Docker services communicate with one another by
service name on the Compose `default` network.

## Intended exposure

| Host port | Binding | Purpose |
| --- | --- | --- |
| `22/tcp` | Public, restricted by the host/cloud firewall | SSH administration |
| `80/tcp`, `443/tcp` | Public | Nginx HTTP/HTTPS entry points |
| `3063/tcp` | `127.0.0.1` only | Nginx upstream for the API |
| `3064/tcp` | `127.0.0.1` only | Host-local metrics scraping |
| `3065/tcp` | `127.0.0.1` only | Nginx upstream for Airflow |
| `3066/tcp` | `127.0.0.1` only | Nginx upstream for MCP |
| `3067/tcp` | `127.0.0.1` only | Grafana dashboard via SSH tunnel |
| `3068/tcp` | `127.0.0.1` only | Prometheus query and readiness |
| `3069/tcp` | `127.0.0.1` only | Alertmanager readiness and test alerts |

MySQL (`3306`), Redis (`6379`), and email-compose gRPC (`50051`)
have no host port mapping. Containers reach them as `db:3306`, `redis:6379`,
and `email-compose:50051`.

The host and cloud firewalls should deny unsolicited inbound traffic by
default. Allow `80/tcp` and `443/tcp` publicly, and limit `22/tcp` to trusted
administration sources where practical. Ports `3060` through `3069` must not
be allowed from external networks.

## Verification

Validate the rendered Compose configuration and the repository's exposure
policy before deployment:

```bash
docker compose --file infra/docker-compose.yml --file infra/docker-compose.prod.yml config --quiet
infra/ops/test-port-exposure.sh
```

After deployment, confirm the host bindings:

```bash
docker compose --file infra/docker-compose.yml --file infra/docker-compose.prod.yml ps
ss -lnt
```

The Docker mappings for `3063` through `3069` must show `127.0.0.1`; there
must be no host mappings for MySQL, Redis, or email-compose. From a machine
outside the production network, scan the formerly published range:

```bash
for port in 3060 3061 3063 3064 3065 3066 3067 3068 3069; do
  nc -z -w 2 <production-host> "$port" && echo "$port unexpectedly open"
done
```

Finally, verify the public HTTPS endpoints through Nginx. Do not test them by
connecting directly to their loopback-only upstream ports.
