package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/urfave/cli"
)

func parseVars(vars []string) map[string]string {
	args := make(map[string]string)

	for i := range vars {
		value := vars[i]
		varVals := strings.Split(strings.TrimSpace(value), "=")
		args[varVals[0]] = varVals[1]
	}

	return args
}

func loadYaml(filePath string, vars []string) YamlSpec {
	args := parseVars(vars)
	return loadFile(filePath, args)
}

func main() {
	app := cli.NewApp()

	app.Commands = []cli.Command{
		cli.Command{
			Name:        "up",
			Usage:       "up [options] /path/to/template.yml",
			UsageText:   "the command to start the pod",
			Description: "Extra details here",
			Flags: []cli.Flag{
				cli.StringSliceFlag{Name: "var"},
				cli.BoolFlag{
					Name: "background",
				},
			},
			Action: func(c *cli.Context) error {
				verbose := c.GlobalBool("verbose")
				norun := c.GlobalBool("norun")
				vars := c.StringSlice("var")
				background := c.Bool("background")
				template := c.Args().First()
				yaml := loadYaml(template, vars)
				command := up(yaml, background)

				if verbose || norun {
					fmt.Println(command)
				}
				rktBinary := c.GlobalString("rkt")

				if !norun {
					syscall.Exec(rktBinary, []string{rktBinary, command}, []string{"4", "5"})
				}
				return nil
			},
		},

		cli.Command{
			Name:        "oneshot",
			Usage:       "oneshot [options] /path/to/template.yml",
			UsageText:   "A command to run in the context of a pod",
			Description: "Extra details here",
			Flags: []cli.Flag{
				cli.StringSliceFlag{Name: "var"},
				cli.StringFlag{Name: "app"},
				cli.StringFlag{Name: "cmd"},
				cli.StringFlag{Name: "name"},
			},
			Action: func(c *cli.Context) error {
				verbose := c.GlobalBool("verbose")
				norun := c.GlobalBool("norun")
				vars := c.StringSlice("var")
				app := c.String("app")
				cmd := c.String("cmd")
				name := c.String("name")
				template := c.Args().First()
				yaml := loadYaml(template, vars)

				if app == "" {
					fmt.Println("Must specify an --app")
					return nil
				}

				command := ""
				if name == "" && cmd == "" {
					fmt.Println("Must specify either --cmd or --name")
					return nil
				} else if name != "" && cmd != "" {
					fmt.Println("Can't specify both --cmd and --name options")
					return nil
				} else if cmd != "" {
					command = oneshotCmd(yaml, app, cmd)
				} else {
					command = oneshotByName(yaml, app, name)
				}

				if verbose || norun {
					fmt.Println(command)
				}

				rktBinary := c.GlobalString("rkt")

				if !norun {
					syscall.Exec(rktBinary, []string{rktBinary, command}, []string{"4", "5"})
				}
				return nil
			},
		},
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "rkt",
			Usage: "Path to rkt binary",
			Value: "/usr/bin/rkt",
		},
		cli.BoolFlag{
			Name:  "norun",
			Usage: "Print the command without running it",
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Print the command before running it",
		},
	}

	app.Run(os.Args)
}
