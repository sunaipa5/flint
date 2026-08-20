package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type TemplateConfig struct {
	Vars map[string]string `yaml:"vars"`
	Cmds []string          `yaml:"cmds"`
}

type TemplateVars map[string]string

type tplStyles struct {
	title        lipgloss.Style
	item         lipgloss.Style
	selectedItem lipgloss.Style
	pagination   lipgloss.Style
	help         lipgloss.Style
}

func newTplStyles() tplStyles {
	var s tplStyles
	s.title = lipgloss.NewStyle().MarginLeft(0).Bold(true).Foreground(lipgloss.Color("#FF5F87"))
	s.item = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#FAFAFA"))
	s.selectedItem = lipgloss.NewStyle().PaddingLeft(0).Bold(true).Foreground(lipgloss.Color("#00FFD7"))
	s.pagination = list.DefaultStyles().PaginationStyle.PaddingLeft(2)
	s.help = list.DefaultStyles().HelpStyle.PaddingLeft(2).PaddingBottom(1)
	return s
}

type tplItem string

func (i tplItem) FilterValue() string { return "" }

type tplItemDelegate struct {
	styles *tplStyles
}

func (d *tplItemDelegate) Height() int                             { return 1 }
func (d *tplItemDelegate) Spacing() int                            { return 0 }
func (d *tplItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d *tplItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(tplItem)
	if !ok {
		return
	}

	str := string(i)
	fn := d.styles.item.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.styles.selectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type tplModel struct {
	list     list.Model
	choice   string
	styles   tplStyles
	quitting bool
}

func initialTplModel(templates []string) tplModel {
	items := make([]list.Item, len(templates))
	for i, t := range templates {
		items[i] = tplItem(t)
	}

	const defaultWidth = 30

	l := list.New(items, &tplItemDelegate{}, defaultWidth, 0)
	l.Title = "Select a template:"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	m := tplModel{list: l}
	m.updateStyles()
	m.updateHelpKeys()
	return m
}

func (m *tplModel) updateStyles() {
	m.styles = newTplStyles()
	m.list.Styles.Title = m.styles.title
	m.list.Styles.PaginationStyle = m.styles.pagination
	m.list.Styles.HelpStyle = m.styles.help
	m.list.SetDelegate(&tplItemDelegate{styles: &m.styles})
}

func (m *tplModel) updateHelpKeys() {
	if m.list.Paginator.TotalPages > 1 {
		m.list.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{
				key.NewBinding(
					key.WithKeys("left", "right", "h", "l"),
					key.WithHelp("←/→", "prev/next page"),
				),
			}
		}
	} else {
		m.list.AdditionalShortHelpKeys = nil
	}
}

func (m tplModel) Init() tea.Cmd {
	return nil
}

func (m tplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if i, ok := m.list.SelectedItem().(tplItem); ok {
				m.choice = string(i)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.updateHelpKeys()
	return m, cmd
}

func (m tplModel) View() string {
	if m.choice != "" || m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

func getTemplatesDir(configDir string) string {
	return filepath.Join(configDir, "templates")
}

func (a *App) ensureTemplatesDir() error {
	templatesDir := getTemplatesDir(a.ConfigDir)
	return os.MkdirAll(templatesDir, 0755)
}

func (a *App) cmdTemplate(args []string) {
	if len(args) == 0 {
		a.cmdTemplateInteractive()
		return
	}

	switch args[0] {
	case "list":
		a.cmdTemplateList()
	case "dir":
		a.cmdTemplateDir()
	case "init":
		a.cmdTemplateInit(args[1:])
	case "new":
		if len(args) < 3 {
			fmt.Println("Usage: flint tpl new <template> <name> [-v KEY=VALUE]")
			return
		}
		a.cmdTemplateNew(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown template command: %s\nRun 'flint tpl help' for usage.\n", args[0])
	}
}

func (a *App) cmdTemplateInteractive() {
	templates, err := a.templates()
	if err != nil || len(templates) == 0 {
		fmt.Println("No templates found.")
		return
	}

	m := initialTplModel(templates)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running template selector: %v\n", err)
		return
	}

	model, ok := finalModel.(tplModel)
	if !ok || model.choice == "" {
		return
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter project name for template '%s': ", model.choice)
	projectName, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading project name: %v\n", err)
		return
	}

	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		fmt.Println("Project name cannot be empty.")
		return
	}

	a.cmdTemplateNew([]string{model.choice, projectName})
}

func (a *App) cmdTemplateList() {
	templates, err := a.templates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading templates: %v\n", err)
		return
	}

	for _, tpl := range templates {
		fmt.Println(tpl)
	}
}

func (a *App) cmdTemplateDir() {
	if err := a.ensureTemplatesDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error accessing templates directory: %v\n", err)
		return
	}
	openOSNative(getTemplatesDir(a.ConfigDir))
}

func (a *App) cmdTemplateInit(args []string) {
	initCmd := flag.NewFlagSet("init", flag.ContinueOnError)
	if err := initCmd.Parse(args); err != nil {
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
		return
	}

	templateYml := filepath.Join(cwd, "template.yml")
	if _, err := os.Stat(templateYml); err == nil {
		fmt.Println("template.yml already exists in current directory.")
		return
	}

	config := TemplateConfig{
		Vars: map[string]string{
			"author": "Your Name",
		},
		Cmds: []string{
			"git init",
			"echo 'Project %n (%p) created by {{.AUTHOR}} on %d (%y)'",
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal template config: %v\n", err)
		return
	}

	if err := os.WriteFile(templateYml, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write template.yml: %v\n", err)
		return
	}

	fmt.Println("Created template.yml in current directory.")
}

func (a *App) cmdTemplateNew(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: flint tpl new <template> <name> [-p <path>] [-v KEY=VALUE]")
		return
	}

	templateName := args[0]
	projectName := args[1]

	var targetDir string
	var varOverrides []string

	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-p", "--path":
			if i+1 < len(args) {
				targetDir = args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s requires a PATH argument\n", arg)
				return
			}
		case "-v", "--var":
			if i+1 < len(args) {
				varOverrides = append(varOverrides, args[i+1])
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s requires a KEY=VALUE argument\n", arg)
				return
			}
		}
	}

	if targetDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
			return
		}
		targetDir = cwd
	}

	templatesDir := getTemplatesDir(a.ConfigDir)
	templateDir := filepath.Join(templatesDir, templateName)
	templateYmlPath := filepath.Join(templateDir, "template.yml")

	if _, err := os.Stat(templateYmlPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Template '%s' not found.\n", templateName)
		return
	}

	configData, err := os.ReadFile(templateYmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read template.yml: %v\n", err)
		return
	}

	var templateConfig TemplateConfig
	if err := yaml.Unmarshal(configData, &templateConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse template.yml: %v\n", err)
		return
	}

	targetPath := filepath.Join(targetDir, projectName)
	if _, err := os.Stat(targetPath); err == nil {
		fmt.Fprintf(os.Stderr, "Target path '%s' already exists.\n", targetPath)
		return
	}

	vars := buildTemplateVars(templateConfig.Vars, projectName, targetPath)
	for _, override := range varOverrides {
		if err := applyVarOverrides(vars, override, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to apply variable overrides: %v\n", err)
			return
		}
	}

	if err := copyTemplate(templateDir, targetPath, vars, []string{"template.yml"}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to copy template: %v\n", err)
		return
	}

	for _, cmd := range templateConfig.Cmds {
		expandedCmd := substituteVars(cmd, vars)
		if err := executeCommandInDir(expandedCmd, targetPath); err != nil {
			fmt.Fprintf(os.Stderr, "Command failed: %s\nError: %v\n", expandedCmd, err)
			return
		}
	}

	if err := a.addProject(projectName, targetPath, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Project created but failed to register: %v\n", err)
		return
	}

	fmt.Printf("Project '%s' created from template '%s' at %s\n", projectName, templateName, targetPath)
}

func buildTemplateVars(templateVars map[string]string, name, projectDir string) TemplateVars {
	now := time.Now()
	vars := make(TemplateVars)

	for k, v := range templateVars {
		vars[strings.ToUpper(k)] = v
	}

	vars["NAME"] = name
	vars["PROJECT_DIR"] = projectDir
	vars["DATE"] = now.Format("2006-01-02")
	vars["YEAR"] = now.Format("2006")

	return vars
}

func applyVarOverrides(vars TemplateVars, shortOverride, longOverride string) error {
	overrides := []string{shortOverride, longOverride}
	for _, override := range overrides {
		if override == "" {
			continue
		}
		parts := strings.SplitN(override, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid variable override format: %s (expected KEY=VALUE)", override)
		}
		vars[strings.ToUpper(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	return nil
}

func copyTemplate(src, dst string, vars TemplateVars, exclude []string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		for _, ex := range exclude {
			if relPath == ex {
				return nil
			}
		}

		newRelPath := substituteVars(relPath, vars)
		targetPath := filepath.Join(dst, newRelPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		newContent := substituteVars(string(content), vars)
		return os.WriteFile(targetPath, []byte(newContent), 0644)
	})
}

func substituteVars(input string, vars TemplateVars) string {
	result := input

	tmpl, err := template.New("subst").Option("missingkey=zero").Parse(input)
	if err == nil {
		var buf strings.Builder
		if err := tmpl.Execute(&buf, vars); err == nil {
			result = buf.String()
		}
	}

	for k, v := range vars {
		placeholder := "%" + strings.ToLower(k)
		result = strings.ReplaceAll(result, placeholder, v)

		placeholderUpper := "%" + strings.ToUpper(k)
		result = strings.ReplaceAll(result, placeholderUpper, v)
	}

	result = strings.ReplaceAll(result, "%n", vars["NAME"])
	result = strings.ReplaceAll(result, "%p", vars["PROJECT_DIR"])
	result = strings.ReplaceAll(result, "%d", vars["DATE"])
	result = strings.ReplaceAll(result, "%y", vars["YEAR"])

	return result
}

func executeCommandInDir(commandLine, dir string) error {
	if strings.TrimSpace(commandLine) == "" {
		return nil
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", commandLine)
	} else {
		cmd = exec.Command("sh", "-c", commandLine)
	}

	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	applySysProcAttr(cmd)
	return cmd.Run()
}

func (a *App) templates() ([]string, error) {
	if err := a.ensureTemplatesDir(); err != nil {
		return nil, err
	}

	templatesDir := getTemplatesDir(a.ConfigDir)
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, err
	}

	var templates []string
	for _, entry := range entries {
		if entry.IsDir() {
			templateYml := filepath.Join(templatesDir, entry.Name(), "template.yml")
			if _, err := os.Stat(templateYml); err == nil {
				templates = append(templates, entry.Name())
			}
		}
	}
	return templates, nil
}

func (a *App) cmdTemplatesJSON() {
	templates, err := a.templates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func (a *App) cmdTemplatesPath() {
	projectsPath := filepath.Join(a.ConfigDir, "templates")
	fmt.Println(projectsPath)
}
