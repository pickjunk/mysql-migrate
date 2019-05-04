package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	migrate "github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	bgo "github.com/pickjunk/bgo"
	dbr "github.com/pickjunk/bgo/dbr"
	bcrypt "golang.org/x/crypto/bcrypt"
	cli "gopkg.in/urfave/cli.v1"
)

func runMigrate(c *cli.Context, callback func(m *migrate.Migrate)) (uint, uint) {
	dir, ok := bgo.Config["migrations"].(string)
	if !ok {
		dir = "migrations"
	}
	mysql := bgo.Config["mysql"].(map[interface{}]interface{})
	dsn := mysql["dsn"].(string)

	m, err := migrate.New("file://"+dir, "mysql://"+dsn)
	if err != nil {
		panic(err)
	}

	oldVersion, _, _ := m.Version()

	callback(m)

	newVersion, _, err := m.Version()
	if err != nil {
		panic(err)
	}

	return oldVersion, newVersion
}

// MigrateCreate migrate create
func MigrateCreate(c *cli.Context) error {
	if !c.Args().Present() {
		cli.ShowCommandHelpAndExit(c, "create", 0)
	}

	dir := bgo.Config["migrations"].(string)
	timestamp := time.Now().Unix()
	base := fmt.Sprintf("%v/%v_%v.", dir, timestamp, c.Args().First())

	os.MkdirAll(dir, os.ModePerm)

	upFile := base + "up.sql"
	if _, err := os.Create(upFile); err != nil {
		panic(err)
	}
	bgo.Log.WithField("name", upFile).Info("migrate create")

	downFile := base + "down.sql"
	if _, err := os.Create(base + "down.sql"); err != nil {
		panic(err)
	}
	bgo.Log.WithField("name", downFile).Info("migrate create")

	return nil
}

// MigrateUp migrate up
func MigrateUp(c *cli.Context) error {
	o, n := runMigrate(c, func(m *migrate.Migrate) {
		err := m.Up()
		if err != nil {
			if err.Error() != "no change" {
				panic(err)
			}
		}
	})

	bgo.Log.WithField("from", o).WithField("to", n).Info("migrate up")

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
			panic(err)
		}
		err = m.Migrate(uint(v))
		if err != nil {
			if err.Error() != "no change" {
				panic(err)
			}
		}
	})

	bgo.Log.WithField("from", o).WithField("to", n).Info("migrate rollback")

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
			panic(err)
		}
		err = m.Force(v)
		if err != nil {
			panic(err)
		}
	})

	bgo.Log.WithField("from", o).WithField("to", n).Info("migrate force")

	return nil
}

// Root insert root or update root
func Root(c *cli.Context) error {
	conn := dbr.New()
	db := conn.NewSession(nil)
	root := bgo.Config["root"].(map[interface{}]interface{})

	table := root["table"].(string)
	name := root["name"].(string)
	passwd := root["passwd"].(string)
	hash, err := bcrypt.GenerateFromPassword([]byte(passwd), 10)
	if err != nil {
		panic(err)
	}
	password := string(hash)

	now := time.Now().Unix()

	skip := []string{"table", "name", "passwd"}
	keys := []string{"name", "passwd"}
	values := []interface{}{name, password}
	for k, v := range root {
		key := k.(string)
		shouldSkip := false
		for _, kk := range skip {
			if key == kk {
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}

		keys = append(keys, key)
		if value, ok := v.(string); ok && value == "now" {
			v = now
		}
		values = append(values, v)
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
			panic(err)
		}

		id, err := r.LastInsertId()
		if err != nil {
			panic(err)
		}

		_, err = db.Update(table).
			Where("id = ?", id).
			Set("id", 1).
			Exec()
		if err != nil {
			panic(err)
		}

		bgo.Log.Info("root insert success")
	} else {
		builder := db.Update(table).
			Where("id = ?", 1)
		for i, k := range keys {
			builder.Set(k, values[i])
		}

		_, err = builder.Exec()
		if err != nil {
			panic(err)
		}

		bgo.Log.Info("root update success")
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
		panic(err)
	}
}
