// Package deps pins the module dependencies while the packages that use them are written.
package deps

import (
	_ "github.com/go-git/go-git/v5"
	_ "github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/yuin/goldmark"
	_ "golang.org/x/crypto/argon2"
	_ "gopkg.in/yaml.v3"
)
