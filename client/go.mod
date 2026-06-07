// Publishable Go client SDK for go-oikumenea — a nested module so external code can
// `go get github.com/olegamysk/go-oikumenea/client`. The package tree under oikumenea/<module> is
// generated from the same api/*.conjure.yml contract as the server (D-Conjure), so it cannot drift.
// Tag releases as client/vX.Y.Z. See client/README.md.
module github.com/olegamysk/go-oikumenea/client

go 1.26.0

require (
	github.com/palantir/conjure-go-runtime/v2 v2.99.0
	github.com/palantir/pkg/bearertoken v1.2.0
	github.com/palantir/pkg/datetime v1.4.0
	github.com/palantir/pkg/safejson v1.2.0
	github.com/palantir/pkg/safeyaml v1.2.0
	github.com/palantir/pkg/uuid v1.3.0
	github.com/palantir/witchcraft-go-error v1.46.0
)

require (
	github.com/golang/snappy v1.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/palantir/go-metrics v1.1.1 // indirect
	github.com/palantir/pkg v1.1.0 // indirect
	github.com/palantir/pkg/bytesbuffers v1.3.0 // indirect
	github.com/palantir/pkg/metrics v1.9.0 // indirect
	github.com/palantir/pkg/refreshable v1.6.0 // indirect
	github.com/palantir/pkg/refreshable/v2 v2.2.0 // indirect
	github.com/palantir/pkg/retry v1.3.0 // indirect
	github.com/palantir/pkg/tlsconfig v1.4.0 // indirect
	github.com/palantir/pkg/transform v1.2.0 // indirect
	github.com/palantir/witchcraft-go-logging v1.63.0 // indirect
	github.com/palantir/witchcraft-go-params v1.41.0 // indirect
	github.com/palantir/witchcraft-go-tracing v1.41.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
