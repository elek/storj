#!/usr/bin/env bash
set -euxo pipefail
cd "$(dirname ${BASH_SOURCE[0]})"/../..

function teardown() {
  ./scripts/unit/teardown.sh
  ./scripts/unit/report.sh
}

./scripts/unit/execute.sh
