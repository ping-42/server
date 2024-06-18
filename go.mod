module github.com/ping-42/server

go 1.22

require (
	github.com/containerd/log v0.1.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/gobwas/ws v1.4.0
	github.com/google/uuid v1.6.0
	github.com/jessevdk/go-flags v1.6.1
	github.com/ping-42/42lib v0.1.15
	github.com/sirupsen/logrus v1.9.3
	gorm.io/gorm v1.25.10
)

require (
	github.com/docker/docker v27.0.0+incompatible // indirect
	github.com/go-gormigrate/gormigrate/v2 v2.1.2 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	gorm.io/driver/postgres v1.5.9 // indirect
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/miekg/dns v1.1.61 // indirect
	golang.org/x/crypto v0.24.0 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
)

// replace github.com/ping-42/42lib => ../42lib
