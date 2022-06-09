#!/usr/bin/env bash
cd "$(dirname ${BASH_SOURCE[0]})"/../..

docker compose -f docker-compose.tests.yaml down -v --remove-orphans ## cleanup previous data
docker compose -f docker-compose.tests.yaml up -d
sleep 3
docker compose -f docker-compose.tests.yaml exec crdb1 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
docker compose -f docker-compose.tests.yaml exec crdb2 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
docker compose -f docker-compose.tests.yaml exec crdb3 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
docker compose -f docker-compose.tests.yaml exec crdb4 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
docker compose -f docker-compose.tests.yaml exec crdb5 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
docker compose -f docker-compose.tests.yaml exec crdb4 bash -c 'cockroach sql --insecure -e "create database testmetabase;"'
docker compose -f docker-compose.tests.yaml exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database teststorj;"'
docker compose -f docker-compose.tests.yaml exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database testmetabase;"'
docker compose -f docker-compose.tests.yaml exec postgres bash -c 'echo "postgres" | psql -U postgres -c "ALTER ROLE postgres CONNECTION LIMIT -1;"'