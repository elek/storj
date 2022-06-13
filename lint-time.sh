set -euxo pipefail
go clean -cache
go clean -testcache
rm -rf ~/.cache/golangci-lint/
time go test -v -p 16 -tags noembed -bench XYZXYZXYZXYZ -run XYZXYZXYZXYZ ./...
time go test -v -p 16 -tags noembed -bench XYZXYZXYZXYZ -run XYZXYZXYZXYZ ./... -race
time golangci-lint --config /home/elek/j/ci/.golangci.yml -j=2 run

go clean -cache
go clean -testcache
rm -rf ~/.cache/golangci-lint/
time golangci-lint --config /home/elek/j/ci/.golangci.yml -j=2 run

