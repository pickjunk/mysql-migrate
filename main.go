package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	migrate "github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	b "github.com/pickjunk/bgo"
	dbr "github.com/pickjunk/bgo/dbr"
	bcrypt "golang.org/x/crypto/bcrypt"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	dir string
	dsn string
)

func init() {
	dir = b.Config.Get("migrations").String()
	if dir == "" {
		dir = "migrations"
	}

	dsn := b.Config.Get("mysql.dsn").String()
	if dsn == "" {
		dsn = "localhost:3306"
	}
}

func runMigrate(c *cli.Context, callback func(m *migrate.Migrate)) (uint, uint) {
	m, err := migrate.New("file://"+dir, "mysql://"+dsn)
	if err != nil {
		log.Panic().Err(err).Send()
	}

	oldVersion, _, _ := m.Version()

	callback(m)

	newVersion, _, err := m.Version()
	if err != nil {
		log.Panic().Err(err).Send()
	}

	return oldVersion, newVersion
}

// MigrateCreate migrate create
func MigrateCreate(c *cli.Context) error {
	if !c.Args().Present() {
		cli.ShowCommandHelpAndExit(c, "create", 0)
	}

	timestamp := time.Now().Unix()
	base := fmt.Sprintf("%v/%v_%v.", dir, timestamp, c.Args().First())

	os.MkdirAll(dir, os.ModePerm)

	upFile := base + "up.sql"
	if _, err := os.Create(upFile); err != nil {
		log.Panic().Err(err).Send()
	}
	log.Info().Str("name", upFile).Msg("migrate create")

	downFile := base + "down.sql"
	if _, err := os.Create(base + "down.sql"); err != nil {
		log.Panic().Err(err).Send()
	}
	log.Info().Str("name", downFile).Msg("migrate create")

	return nil
}

// MigrateUp migrate up
func MigrateUp(c *cli.Context) error {
	o, n := runMigrate(c, func(m *migrate.Migrate) {
		err := m.Up()
		if err != nil {
			if err.Error() != "no change" {
				log.Panic().Err(err).Send()
			}
		}
	})

	log.Info().Uint("from", o).Uint("to", n).Msg("migrate up")

	return nil
}

// MigrateRollback migrate rollback
func MigrateRollback(c *cli.Context) error {
	if !c.Args().Present() {
		cli.ShowCommandHelpAndExit(c, "rollback", 0)
	}

	o, n := runMigrate(c, func(m *migrate.Migrate) {
		v, err := strconv.Atoi(c.Args().First())
		if err != nil {
			log.Panic().Err(err).Send()
		}
		err = m.Migrate(uint(v))
		if err != nil {
			if err.Error() != "no change" {
				log.Panic().Err(err).Send()
			}
		}
	})

	log.Info().Uint("from", o).Uint("to", n).Msg("migrate rollback")

	return nil
}

// MigrateForce migrate force
func MigrateForce(c *cli.Context) error {
	if !c.Args().Present() {
		cli.ShowCommandHelpAndExit(c, "force", 0)
	}

	o, n := runMigrate(c, func(m *migrate.Migrate) {
		v, err := strconv.Atoi(c.Args().First())
		if err != nil {
			log.Panic().Err(err).Send()
		}
		err = m.Force(v)
		if err != nil {
			log.Panic().Err(err).Send()
		}
	})

	log.Info().Uint("from", o).Uint("to", n).Msg("migrate force")

	return nil
}

// Root insert root or update root
func Root(c *cli.Context) error {
	conn := dbr.New()
	db := conn.NewSession(nil)

	root := b.Config.Get("root").Map()
	table := b.Config.Get("root.table").String()
	name := b.Config.Get("root.name").String()
	passwd := b.Config.Get("root.passwd").String()
	hash, err := bcrypt.GenerateFromPassword([]byte(passwd), 10)
	if err != nil {
		log.Panic().Err(err).Send()
	}
	password := string(hash)

	now := time.Now().Unix()

	skip := []string{"table", "name", "passwd"}
	keys := []string{"name", "passwd"}
	values := []interface{}{name, password}
	for k, v := range root {
		shouldSkip := false
		for _, kk := range skip {
			if k == kk {
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}

		keys = append(keys, k)
		if value := v.String(); value == "now" {
			values = append(values, now)
		} else {
			values = append(values, v.Value())
		}
	}

	var exist struct{}
	err = db.Select("*").
		From(table).
		Where("id = ?", 1).
		LoadOne(&exist)
	if err != nil {
		r, err := db.InsertInto(table).
			Columns(keys...).
			Values(values...).
			Exec()
		if err != nil {
			log.Panic().Err(err).Send()
		}

		id, err := r.LastInsertId()
		if err != nil {
			log.Panic().Err(err).Send()
		}

		_, err = db.Update(table).
			Where("id = ?", id).
			Set("id", 1).
			Exec()
		if err != nil {
			log.Panic().Err(err).Send()
		}

		log.Info().Msg("root insert success")
	} else {
		builder := db.Update(table).
			Where("id = ?", 1)
		for i, k := range keys {
			builder.Set(k, values[i])
		}

		_, err = builder.Exec()
		if err != nil {
			log.Panic().Err(err).Send()
		}

		log.Info().Msg("root update success")
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "mysql-migrate"
	app.Usage = "mysql migration tool"

	app.Commands = []cli.Command{
		{
			Name:  "migrate",
			Usage: "run migrations",
			Subcommands: []cli.Command{
				{
					Name:      "create",
					ArgsUsage: "[name]",
					Usage:     "create a new migration",
					Action:    MigrateCreate,
				},
				{
					Name:   "up",
					Usage:  "apply all migrations to the latest one",
					Action: MigrateUp,
				},
				{
					Name:      "rollback",
					ArgsUsage: "[version]",
					Usage:     "rollback to the specific version",
					Action:    MigrateRollback,
				},
				{
					Name:      "force",
					ArgsUsage: "[version]",
					Usage:     "force to the specific version",
					Action:    MigrateForce,
				},
			},
		},
		{
			Name:   "root",
			Usage:  "insert root, or reset root password",
			Action: Root,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Panic().Err(err).Send()
	}
}
