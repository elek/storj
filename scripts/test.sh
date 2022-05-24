#!/usr/bin/env bash

set -ex
TEST_ARGS=$@

if [ ! $TEST_ARGS ]; then
   TEST_ARGS=./...
fi

cd "$(dirname ${BASH_SOURCE[0]})"/..
rm -rf .build || true
mkdir .build
go clean -testcache

docker-compose down -v
docker-compose up -d
sleep 10

export STORJ_TEST_HOST='127.0.0.20;127.0.0.21;127.0.0.22;127.0.0.23;127.0.0.24;127.0.0.25'
export STORJ_TEST_COCKROACH='cockroach://root@localhost:26256/testcockroach?sslmode=disable;cockroach://root@localhost:26257/testcockroach?sslmode=disable;cockroach://root@localhost:26258/testcockroach?sslmode=disable;cockroach://root@localhost:26259/testcockroach?sslmode=disable'
export STORJ_TEST_COCKROACH_ALT='cockroach://root@localhost:26260/testcockroach?sslmode=disable'
export STORJ_TEST_POSTGRES='postgres://storjtest@localhost/teststorj?sslmode=disable'
export STORJ_TEST_LOG_LEVEL='info'

cockroach sql --insecure --host=localhost:26256 -e 'create database testcockroach;'
cockroach sql --insecure --host=localhost:26257 -e 'create database testcockroach;'
cockroach sql --insecure --host=localhost:26258 -e 'create database testcockroach;'
cockroach sql --insecure --host=localhost:26259 -e 'create database testcockroach;'
cockroach sql --insecure --host=localhost:26260 -e 'create database testcockroach;'
cockroach sql --insecure --host=localhost:26259 -e 'create database testmetabase;'

psql -h localhost -U storjtest -c 'create database teststorj;'
psql -h localhost -U storjtest -c 'create database testmetabase;'
psql -h localhost -U storjtest -c 'ALTER ROLE storjtest CONNECTION LIMIT -1;'

set +e #following may fail, we should try to do all the final steps
go test -tags noembed -vet=off $COVERFLAGS -p 16 -parallel 1 -timeout 32m -json -race $TEST_ARGS 2>&1 | tee .build/tests.json | xunit -out .build/tests.xml
cat .build/tests.json | tparse -slow 20 -top
docker-compose down -v
