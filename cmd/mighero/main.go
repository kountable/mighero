package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
	_ "github.com/gemnasium/migrate/driver/bash"
	_ "github.com/gemnasium/migrate/driver/cassandra"
	_ "github.com/gemnasium/migrate/driver/mysql"
	_ "github.com/gemnasium/migrate/driver/postgres"
	"github.com/gemnasium/migrate/file"
	"github.com/gemnasium/migrate/migrate"
	"github.com/gemnasium/migrate/migrate/direction"
	pipep "github.com/gemnasium/migrate/pipe"
	"gopkg.in/yaml.v2"
)

const (
	cmdCreate  = "create"
	cmdMigrate = "migrate"
	cmdHelp    = "help"
	cmdVersion = "version"
	cmdUp      = "up"
	cmdDown    = "down"
	cmdReset   = "reset"
	cmdRedo    = "redo"
	cmdGoto    = "goto"
)

var path struct {
	ToDefault string
	ToEnv     string
	Section   string
}

func main() {
	flag.Usage = func() {
		helpCmd()
	}
	flag.StringVar(&path.ToDefault, "def", "env/default.yml", "the default configuration file.")
	flag.StringVar(&path.ToEnv, "env", "env/docker.yml", "the environment configuration file.")
	flag.StringVar(&path.Section, "section", "db", "the section from config file")
	flag.Parse()

	fmt.Println("--------------------------------")
	fmt.Println("Def config -", path.ToDefault)
	fmt.Println("Env config -", path.ToEnv)
	fmt.Println("Section -", path.Section)
	fmt.Println("--------------------------------")
	fmt.Println()

	cmd := flag.Arg(0)

	c, err := initConfig(path.ToDefault, path.ToEnv, path.Section)
	if err != nil {
		log.Printf("%s, trying environment variables", err)
		c = map[string]interface{}{}
	}

	//TODO: use env lib to store in a struct
	dbUserFromEnv := os.Getenv("K_DB_USER")
	dbPasswordFromEnv := os.Getenv("K_DB_PASSWORD")
	dbIPFromEnv := os.Getenv("K_DB_IP")
	dbPortFromEnv := os.Getenv("K_DB_PORT")
	dbNameFromEnv := os.Getenv("K_DB_NAME")

	dbTestUserFromEnv := os.Getenv("K_TEST_MODE_DB_USER")
	dbTestPasswordFromEnv := os.Getenv("K_TEST_MODE_DB_PASSWORD")
	dbTestIPFromEnv := os.Getenv("K_TEST_MODE_DB_IP")
	dbTestPortFromEnv := os.Getenv("K_TEST_MODE_DB_PORT")
	dbTestNameFromEnv := os.Getenv("K_TEST_MODE_DB_NAME")

	dbRedshiftUserFromEnv := os.Getenv("K_REDSHIFT_USER")
	dbRedshiftPasswordFromEnv := os.Getenv("K_REDSHIFT_PASSWORD")
	dbRedshiftIPFromEnv := os.Getenv("K_REDSHIFT_IP")
	dbRedshiftPortFromEnv := os.Getenv("K_REDSHIFT_PORT")
	dbRedshiftNameFromEnv := os.Getenv("K_REDSHIFT_NAME")
	if path.Section == "db" {
		if len(dbUserFromEnv) > 0 {
			c["user"] = dbUserFromEnv
		}
		if len(dbPasswordFromEnv) > 0 {
			c["password"] = dbPasswordFromEnv
		}

		if len(dbIPFromEnv) > 0 && len(dbPortFromEnv) > 0 {
			c["ip"] = fmt.Sprintf("%s:%s", dbIPFromEnv, dbPortFromEnv)
		}

		if len(dbNameFromEnv) > 0 {
			c["name"] = dbNameFromEnv
		}
	} else if path.Section == "test_mode_db" {
		if len(dbTestUserFromEnv) > 0 {
			c["user"] = dbTestUserFromEnv
		}
		if len(dbTestPasswordFromEnv) > 0 {
			c["password"] = dbTestPasswordFromEnv
		}

		if len(dbTestIPFromEnv) > 0 && len(dbTestPortFromEnv) > 0 {
			c["ip"] = fmt.Sprintf("%s:%s", dbTestIPFromEnv, dbTestPortFromEnv)
		}

		if len(dbTestNameFromEnv) > 0 {
			c["name"] = dbTestNameFromEnv
		}
	} else if path.Section == "redshift" {
		if len(dbRedshiftUserFromEnv) > 0 {
			c["user"] = dbRedshiftUserFromEnv
		}
		if len(dbRedshiftPasswordFromEnv) > 0 {
			c["password"] = dbRedshiftPasswordFromEnv
		}

		if len(dbRedshiftIPFromEnv) > 0 && len(dbRedshiftPortFromEnv) > 0 {
			c["ip"] = fmt.Sprintf("%s:%s", dbRedshiftIPFromEnv, dbRedshiftPortFromEnv)
		}

		if len(dbRedshiftNameFromEnv) > 0 {
			c["name"] = dbRedshiftNameFromEnv
		}
	}

	// Validation parameters
	switch {
	case c["migration_dir"] == nil:
		fmt.Println("Error: please set 'migration_dir' parameter in DB config")
		return
	case c["driver"] == nil:
		fmt.Println("Error: please set 'driver' parameter in DB config")
		return
	case c["user"] == nil:
		fmt.Println("Error: please set 'user' parameter in DB config")
		return
	case c["password"] == nil:
		c["password"] = ""
	case c["ip"] == nil:
		fmt.Println("Error: please set 'ip' parameter in DB config")
		return
	case c["name"] == nil:
		fmt.Println("Error: please set 'name' parameter in DB config")
		return
	}

	driver := c["driver"].(string)
	var url string

	switch driver {
	case "postgres":
		url = fmt.Sprintf("%s://%s:%s@%s/%s?sslmode=disable", driver,
			c["user"].(string),
			c["password"].(string),
			c["ip"].(string),
			c["name"].(string))

	case "mysql":
		url = fmt.Sprintf("%s://%s:%s@tcp(%s)/%s", driver,
			c["user"].(string),
			c["password"].(string),
			c["ip"].(string),
			c["name"].(string))
	default:
		fmt.Printf("Error: unknown driver '%s'\n", driver)
		return

	}

	migrationDir := c["migration_dir"].(string)
	switch cmd {
	case cmdCreate:

		name := flag.Arg(1)
		if name == "" {
			fmt.Println("Please specify name.")
			os.Exit(1)
		}

		migrationFile, err := migrate.Create(url, migrationDir, name)
		switch e := err.(type) {
		case *net.OpError:
			fmt.Println(fmt.Sprintf("Can't connect to the DB: %s. Error: %s", url, e))
		}
		//		pretty.Println("=============    (╯°□°）╯︵ ┻━┻)   =============")
		//		pretty.Println(err)
		//		pretty.Println("=============    ┬─┬ノ( º _ ºノ)   =============")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Version %v migration files created in %v:\n", migrationFile.Version, migrationDir)
		fmt.Println(migrationFile.UpFile.FileName)
		fmt.Println(migrationFile.DownFile.FileName)
		////////// CREATE END ///////////////
	case cmdMigrate:
		relativeN := flag.Arg(1)
		relativeNInt, err := strconv.Atoi(relativeN)
		if err != nil {
			fmt.Println("Unable to parse param <n>.")
			os.Exit(1)
		}
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Migrate(pipe, url, migrationDir, relativeNInt)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
		////////// MIGRATE END ///////////////
	case cmdUp:
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Up(pipe, url, migrationDir)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
		////////// UP END ///////////////
	case cmdDown:
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Down(pipe, url, migrationDir)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

		////////// DOWN END ///////////////
	case cmdVersion:
		version, err := migrate.Version(url, migrationDir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(version)
		////////// VERSION END ///////////////

	case cmdReset:
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Reset(pipe, url, migrationDir)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
		////////// RESET END ///////////////

	case cmdRedo:
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Redo(pipe, url, migrationDir)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
		////////// REDO END ///////////////

	case cmdGoto:
		toVersion := flag.Arg(1)
		toVersionInt, err := strconv.Atoi(toVersion)
		if err != nil || toVersionInt < 0 {
			fmt.Println("Unable to parse param <v>.")
			os.Exit(1)
		}

		currentVersion, err := migrate.Version(url, migrationDir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		relativeNInt := toVersionInt - int(currentVersion)

		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Migrate(pipe, url, migrationDir, relativeNInt)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
		////////// GOTO  END ///////////////

	default:
		fallthrough
	case cmdHelp:
		helpCmd()
	}
}

func writePipe(pipe chan interface{}) (ok bool) {
	okFlag := true
	if pipe != nil {
		for {
			select {
			case item, more := <-pipe:
				if !more {
					return okFlag
				}

				switch item.(type) {
				case string:
					fmt.Println(item.(string))

				case error:
					c := color.New(color.FgRed)
					c.Println(item.(error).Error())
					okFlag = false

				case file.File:
					f := item.(file.File)
					c := color.New(color.FgBlue)
					if f.Direction == direction.Up {
						c.Print(">")
					} else if f.Direction == direction.Down {
						c.Print("<")
					}
					fmt.Printf(" %s\n", f.FileName)

				default:
					text := fmt.Sprint(item)
					fmt.Println(text)
				}
			}
		}
	}
	return okFlag
}

var timerStart time.Time

func printTimer() {
	diff := time.Now().Sub(timerStart).Seconds()
	if diff > 60 {
		fmt.Printf("\n%.4f minutes\n", diff/60)
	} else {
		fmt.Printf("\n%.4f seconds\n", diff)
	}
}

func helpCmd() {
	os.Stderr.WriteString(
		`usage: mighero [-def=<path> -env=<path>] <command> [<args>]
Commands:
   create <name>  Create a new migration
   up             Apply all -up- migrations
   down           Apply all -down- migrations
   reset          Down followed by Up
   redo           Roll back most recent migration, then apply it again
   version        Show current migration version
   migrate <n>    Apply migrations -n|+n
   help           Show this help
'-path' defaults to the subdirectory env of current working directory.
`)
}

func initConfig(defaultConfigPath, envConfigPath, section string) (map[string]interface{}, error) {
	def, err := ioutil.ReadFile(defaultConfigPath)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(def)

	configPart := "config"
	if env, err := ioutil.ReadFile(envConfigPath); err == nil {
		buf.Write(env)
	} else {
		configPart = "default"
		log.Printf("could not read env configuration, proceeding with defaults only: %s", err)
	}

	cMap := map[string]map[string]interface{}{}

	if err = yaml.Unmarshal(buf.Bytes(), cMap); err != nil {
		return nil, err
	}

	if _, ok := cMap[configPart][section]; !ok {
		return nil, fmt.Errorf("section '%s' in config is undefined in config part '%s'", section, configPart)
	}

	getSectionBytes, err := yaml.Marshal(cMap[configPart][section])
	if err != nil {
		return nil, err
	}

	sectionMap := map[string]interface{}{}

	if err = yaml.Unmarshal(getSectionBytes, sectionMap); err != nil {
		return nil, err
	}

	return sectionMap, nil
}
