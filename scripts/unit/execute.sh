#!/usr/bin/env bash
cd "$(dirname ${BASH_SOURCE[0]})"/../..

set -ex

export STORJ_TEST_COCKROACH_NODROP='true'
export STORJ_TEST_POSTGRES='postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable'
export STORJ_TEST_COCKROACH="cockroach://root@localhost:26356/testcockroach?sslmode=disable"
export STORJ_TEST_COCKROACH="$STORJ_TEST_COCKROACH;cockroach://root@localhost:26357/testcockroach?sslmode=disable"
export STORJ_TEST_COCKROACH="$STORJ_TEST_COCKROACH;cockroach://root@localhost:26358/testcockroach?sslmode=disable"
export STORJ_TEST_COCKROACH="$STORJ_TEST_COCKROACH;cockroach://root@localhost:26359/testcockroach?sslmode=disable"
export STORJ_TEST_COCKROACH_ALT='cockroach://root@localhost:26360/testcockroach?sslmode=disable'
export STORJ_TEST_LOG_LEVEL='info'

mkdir -p .build
rm .build/tests.json || true
go test -tags noembed -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $TEST_TARGET -json | tee .build/tests.json