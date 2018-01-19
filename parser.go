package main

import (
	"bytes"
	"html/template"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type NameValuePair struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Meta struct {
	Cli     map[string]string `yaml:"cli"`
	Oneshot map[string]string `yaml:"oneshot"`
}

type Port struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type IsolatorValue struct {
	Set   []string `yaml:"set"`
	Errno string   `yaml:"errno"`
}

type Isolator struct {
	Name  string        `yaml:"name"`
	Value IsolatorValue `yaml:"value"`
}

type AppSpec struct {
	Exec      []string   `yaml:"exec"`
	Ports     []Port     `yaml:"ports"`
	Isolators []Isolator `yaml:"isolators"`
}

type Mount struct {
	Volume string `yaml:"volume"`
	Path   string `yaml:"path"`
}

type App struct {
	Image       string          `yaml:"image"`
	Name        string          `yaml:"name"`
	App         AppSpec         `yaml:"app"`
	Mounts      []Mount         `yaml:"mounts"`
	Environment []NameValuePair `yaml:"environment"`
}

type Volume struct {
	Name   string `yaml:"name"`
	Kind   string `yaml:"kind"`
	Source string `yaml:"source"`
}

type YamlSpec struct {
	Meta    Meta     `yaml:"__meta__"`
	Apps    []App    `yaml:"apps"`
	Volumes []Volume `yaml:"volumes"`
}

func loadFile(filePath string, vars map[string]string) YamlSpec {
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	tmpl, err := template.
		New("manifest").
		Option("missingkey=error").
		Parse(string(fileContents))

	if err != nil {
		panic(err)
	}

	var resolvedYaml bytes.Buffer
	err = tmpl.Execute(&resolvedYaml, vars)
	if err != nil {
		panic(err)
	}

	yamlSpec := YamlSpec{}
	err = yaml.Unmarshal([]byte(resolvedYaml.String()), &yamlSpec)
	if err != nil {
		panic(err)
	}

	if len(yamlSpec.Apps) == 0 {
		panic("Pod manifest must contain at least one app")
	}

	if yamlSpec.Meta.Cli == nil {
		yamlSpec.Meta.Cli = make(map[string]string)
	}
	return yamlSpec
}
