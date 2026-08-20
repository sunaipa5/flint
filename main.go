package main

import (
	"fmt"
	"os"
)

var version string

func main() {
	app, err := newApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing app: %v\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		app.cmdSelect()
		return
	}

	switch args[0] {
	case "help", "-h", "-help", "--help":
		app.cmdHelp()
	case "version", "-v", "--version":
		fmt.Println(version)
	case "list":
		app.cmdList()
	case "select":
		app.cmdSelect()
	case "last":
		app.cmdLast()
	case "open":
		app.cmdOpen(args[1:])
	case "dir":
		app.cmdProjectDir(args)
	case "add":
		app.cmdAdd(args[1:])
	case "init":
		app.cmdInit(args[1:])
	case "edit":
		app.cmdEdit(args[1:])
	case "rm", "remove":
		app.cmdRemove(args[1:])
	case "editor":
		app.cmdEditor(args[1:])
	case "ws":
		app.cmdWorkspace(args[1:])
	case "projects-json":
		app.cmdProjectsJSON()
	case "last-project-json":
		app.cmdLastProjectJSON()
	case "project-json":
		app.cmdProjectJSON(args[1:])
	case "workspaces-json":
		app.cmdWorkspacesJSON()
	case "config-json":
		app.cmdConfigJSON()
	case "last-project-name":
		app.cmdLastProjectName()
	case "last-project-path":
		app.cmdLastProjectPath()
	case "project-path":
		app.cmdProjectPath(args[1:])
	case "config-path":
		app.cmdConfigPath()
	case "workspaces-path":
		app.cmdWorkspacesPath()
	case "projects-path":
		app.cmdProjectsPath()
	case "tpl", "tmpl", "template":
		app.cmdTemplate(args[1:])
	case "templates-json":
		app.cmdTemplatesJSON()
	case "templates-path":
		app.cmdTemplatesPath()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		app.cmdHelp()
		os.Exit(1)
	}
}
