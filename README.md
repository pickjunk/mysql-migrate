# mysql-migrate

Migration tool, base on [golang-migrate/migrate](https://github.com/golang-migrate/migrate).

### Quick Start

```shell
$ go get -v github.com/pickjunk/mysql-migrate
$ mysql-migrate # print help document
```

### Commands

```shell
$ mysql-migrate migrate create [migration-name]
$ mysql-migrate migrate up
$ mysql-migrate migrate rollback [version]
$ mysql-migrate migrate force [version] # use this when version is dirty (after error occurred)

$ mysql-migrate root # insert or update root record
```
