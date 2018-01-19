package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func cliOptions(options map[string]string) []string {
	optionStrings := []string{}
	for name, value := range options {
		flag := fmt.Sprintf("--%v=%v", name, value)
		optionStrings = append(optionStrings, flag)
	}
	sort.Strings(optionStrings)
	return optionStrings
}

func volumeOptions(options []Volume) []string {
	optionStrings := []string{}
	for i := range options {
		flag := fmt.Sprintf(
			"--volume=%v,kind=%v,source=%v",
			options[i].Name,
			options[i].Kind,
			options[i].Source)
		optionStrings = append(optionStrings, flag)
	}
	sort.Strings(optionStrings)
	return optionStrings
}

func appOptions(app App) []string {
	options := []string{}
	options = append(options, fmt.Sprintf("docker://%v", app.Image))

	for i := range app.Environment {
		s := fmt.Sprintf(
			"--environment=%v=%v",
			app.Environment[i].Name,
			app.Environment[i].Value)
		options = append(options, s)
	}

	for i := range app.Mounts {
		s := fmt.Sprintf(
			"--mount=volume=%v,target=%v",
			app.Mounts[i].Volume,
			app.Mounts[i].Path)
		options = append(options, s)
	}

	options = append(options, fmt.Sprintf("--name=%v", app.Name))

	for i := range app.App.Ports {
		s := fmt.Sprintf(
			"--port=%v:%v",
			app.App.Ports[i].Name,
			app.App.Ports[i].Port)
		options = append(options, s)
	}

	for i := range app.App.Isolators {
		iso := app.App.Isolators[i]
		name := iso.Name
		if name == "os/linux/seccomp-retain-set" {
			name = "retain"
		}

		isoOpts := []string{}

		isoOpts = append(isoOpts, fmt.Sprintf("--seccomp=mode=%v", name))
		isoOpts = append(isoOpts, iso.Value.Set...)

		if iso.Value.Errno != "" {
			isoOpts = append(isoOpts, fmt.Sprintf("errno=%v", iso.Value.Errno))
		}

		options = append(options, strings.Join(isoOpts, ","))
	}

	if len(app.App.Exec) > 0 {
		options = append(options, fmt.Sprintf("--exec=%v", app.App.Exec[0]))
	}

	if len(app.App.Exec) > 1 {
		args := strings.Join(app.App.Exec[1:len(app.App.Exec)], " ")
		options = append(options, fmt.Sprintf("-- %v", args))
		options = append(options, "---")
	}

	return options
}

func appsOptions(apps []App) []string {
	appStrings := []string{}
	for i := range apps {
		appStrings = append(appStrings, appOptions(apps[i])...)
	}

	return appStrings
}

func formatCmd(args []string) string {
	indent := 0
	results := []string{}
	for i := range args {
		if args[i] == "&&" {
			indent = 0
		}

		leadingSpace := strings.Repeat("  ", indent)
		results = append(results, fmt.Sprintf("%v%v", leadingSpace, args[i]))

		if args[i] == "---" {
			indent = indent - 1
		} else if string(args[i][len(args[i])-1]) == ";" {
			indent = 0
		} else if string(args[i][0]) != "-" {
			indent = indent + 1
		}
	}

	return strings.Join(results, " \\\n")
}

func upCmd(yaml YamlSpec, background bool, serviceName string) []string {
	cliOptions := cliOptions(yaml.Meta.Cli)
	volumeOptions := volumeOptions(yaml.Volumes)
	appsOptions := appsOptions(yaml.Apps)

	runOpts := []string{}

	if background {
		runOpts = append(runOpts, "systemd-run")
		runOpts = append(runOpts, fmt.Sprintf("--unit=%v", serviceName))
	}

	runOpts = append(runOpts, []string{"rkt", "run"}...)
	runOpts = append(runOpts, cliOptions...)
	runOpts = append(runOpts, volumeOptions...)
	runOpts = append(runOpts, appsOptions...)

	return runOpts
}

func down(unit string) []string {
	cmd := []string{}
	cmd = append(cmd, fmt.Sprintf(
		"systemctl stop %v;",
		unit))
	cmd = append(cmd, fmt.Sprintf(
		"systemctl reset-failed %v 2>/dev/null;",
		unit))

	return cmd
}

func makeUUID() string {
	u := make([]byte, 16)
	_, err := rand.Read(u)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}

	u[8] = (u[8] | 0x80) & 0xBF // what does this do?
	u[6] = (u[6] | 0x40) & 0x4F // what does this do?

	return hex.EncodeToString(u)
}

func makeUnit() string {
	return fmt.Sprintf("rkt-launch-%v", makeUUID())
}

func up(yaml YamlSpec, background bool) string {
	return formatCmd(upCmd(yaml, background, makeUnit()))
}

func makeUUIDFile() string {
	return fmt.Sprintf("/tmp/rkt-launch-%v", makeUUID())
}

func oneshotCmd(yaml YamlSpec, app string, cmd string) string {
	uuidFile := makeUUIDFile()
	yaml.Meta.Cli["uuid-file-save"] = uuidFile
	unit := makeUnit()
	upCmd := upCmd(yaml, true, unit)
	downCmd := down(unit)

	oneshotCmd := fmt.Sprintf(
		"sudo rkt enter --app=%v `cat %v` %v",
		app,
		uuidFile,
		cmd)

	runOpts := []string{}
	runOpts = append(runOpts, upCmd...)
	runOpts = append(runOpts, []string{
		"&&",
		fmt.Sprintf("while [ ! -s %v ]; do sleep 0.2; printf .; done", uuidFile),
		"&&",
		fmt.Sprintf("%v;", oneshotCmd),
		"status=$? ;"}...)
	runOpts = append(runOpts, downCmd...)
	runOpts = append(runOpts, fmt.Sprintf("rm -f %v;", uuidFile))
	runOpts = append(runOpts, "exit $status")
	return formatCmd(runOpts)
}

func oneshotByName(yaml YamlSpec, app string, name string) string {
	return oneshotCmd(yaml, app, yaml.Meta.Oneshot[name])
}
