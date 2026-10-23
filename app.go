package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/v-felipeolliveira/ssm-helper/internal/chrome"
	"github.com/v-felipeolliveira/ssm-helper/internal/env"
	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/v-felipeolliveira/ssm-helper/internal/settings"
	"github.com/v-felipeolliveira/ssm-helper/internal/sso"
	"github.com/v-felipeolliveira/ssm-helper/internal/tunnel"
)

// Tunnel statuses sent to the frontend
const (
	StatusIdle       = "idle"
	StatusConnecting = "connecting"
	StatusLogin      = "login"
	StatusConnected  = "connected"
	StatusError      = "error"
)

// Events emitted to the frontend
const (
	eventTunnelState = "tunnel:state"
	eventLog         = "log"
	eventSSOCode     = "sso:code"
)

// EnvState is the active environment file and its problems.
type EnvState struct {
	// Name is the environment name; empty means env.json.
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Path     string   `json:"path"`
	Env      *env.Env `json:"env"`
	Problems []string `json:"problems"`
	// Error is "notFound", "invalidJson" or "read" when the file could not be loaded.
	Error       string `json:"error"`
	ErrorDetail string `json:"errorDetail"`
}

// SaveResult is the outcome of SaveEnv.
type SaveResult struct {
	Problems []string `json:"problems"`
	State    EnvState `json:"state"`
}

type EnvInfo struct {
	Name          string `json:"name"`
	File          string `json:"file"`
	ActiveTunnels int    `json:"activeTunnels"`
}

// EnvList is the set of environments next to the executable.
type EnvList struct {
	Dir    string    `json:"dir"`
	Active string    `json:"active"`
	Envs   []EnvInfo `json:"envs"`
}

// NewEnvRequest creates env.<name>.json.
type NewEnvRequest struct {
	Name string `json:"name"`
	// CopyCurrent starts from the active environment instead of an empty one.
	CopyCurrent bool `json:"copyCurrent"`
	// RenameCurrentTo names env.json, which is hidden once a named environment exists.
	RenameCurrentTo string `json:"renameCurrentTo"`
}

type AppearanceState struct {
	Appearance settings.Appearance `json:"appearance"`
	Builtins   []settings.Palette  `json:"builtins"`
}

type TunnelState struct {
	Env     string `json:"env"`
	Target  string `json:"target"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ProfileStatus struct {
	Name      string `json:"name"`
	LoggedIn  bool   `json:"loggedIn"`
	ExpiresAt string `json:"expiresAt"`
}

type LogEntry struct {
	Time   string `json:"time"`
	Source string `json:"source"`
	Text   string `json:"text"`
}

type SSOCode struct {
	Profile string         `json:"profile"`
	Code    sso.DeviceCode `json:"code"`
}

// App holds the state shared with the frontend through Wails bindings.
type App struct {
	ctx     context.Context
	dir     string // folder with the environments and ssm-helper.json
	dataDir string // ssm-helper-data next to the executable: SSO cache, Chrome profiles, WebView2 data
	sso     *sso.Manager

	mu       sync.Mutex
	settings settings.Settings
	current  *env.Env // active environment, only when valid
	tunnels  map[string]*tunnel.Session
	states   map[string]TunnelState
	cancels  map[string]context.CancelFunc // pending connects and logins
	startup  []LogEntry                    // messages from before the window opened
}

func NewApp(dir, dataDir string) *App {
	a := &App{
		dir:     dir,
		dataDir: dataDir,
		tunnels: map[string]*tunnel.Session{},
		states:  map[string]TunnelState{},
		cancels: map[string]context.CancelFunc{},
	}
	a.sso = sso.NewManager(filepath.Join(dataDir, "sso-cache"), func(url string) {
		wruntime.BrowserOpenURL(a.ctx, url)
	})
	a.moveLegacyDataDir()
	a.loadSettings()
	return a
}

// loadSettings reads ssm-helper.json and migrates files from older versions:
// config.yaml/config.json become env files, and ssm-helper.settings.json becomes ssm-helper.json.
func (a *App) loadSettings() {
	s, problems, source, err := settings.Load(a.dir)
	if err != nil {
		a.note("settings", fmt.Sprintf("Não foi possível ler %s: %v. Usando os padrões.", settings.FileName, err))
	}
	for _, p := range problems {
		a.note("settings", fmt.Sprintf("%s: %s. A aparência padrão foi usada.", settings.FileName, p))
	}
	save := source == settings.SourceLegacyFile
	if save {
		a.note("settings", fmt.Sprintf("%s convertido para %s (o arquivo antigo foi mantido).", settings.LegacyFileName, settings.FileName))
	}

	migration, err := env.MigrateLegacy(a.dir)
	if err != nil {
		a.note("env", fmt.Sprintf("Falha ao converter os arquivos de configuração antigos: %v", err))
	}
	froms := make([]string, 0, len(migration.Files))
	for from := range migration.Files {
		froms = append(froms, from)
	}
	slices.Sort(froms)
	for _, from := range froms {
		a.note("env", fmt.Sprintf("%s convertido para %s (o arquivo antigo foi mantido).", from, migration.Files[from]))
		save = true
	}
	// The appearance used to live in config.yaml
	if source == settings.SourceNone && len(migration.AppearanceSources) > 0 {
		from := migration.AppearanceSources[0]
		if data, err := os.ReadFile(filepath.Join(a.dir, from)); err == nil {
			if appearance, problems := settings.ParseAppearance(env.AppearanceNode(data)); len(problems) == 0 {
				s.Appearance = appearance
				a.note("settings", fmt.Sprintf("Aparência trazida de %s para %s.", from, settings.FileName))
			}
		}
	}

	names, _ := env.List(a.dir)
	if !slices.Contains(names, s.ActiveEnv) {
		s.ActiveEnv = env.DefaultName
		if len(names) > 0 {
			s.ActiveEnv = names[0]
		}
	}
	a.settings = s
	if save {
		a.saveSettings()
	}
}

func (a *App) startupHook(ctx context.Context) {
	a.ctx = ctx
}

// shutdown closes every tunnel when the window closes.
func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	for _, cancel := range a.cancels {
		cancel()
	}
	sessions := make([]*tunnel.Session, 0, len(a.tunnels))
	for _, s := range a.tunnels {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()
	for _, s := range sessions {
		s.Stop()
	}
}

// StartupMessages returns what happened before the window opened (migration, settings problems).
func (a *App) StartupMessages() []LogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.startup)
}

// ListEnvs returns the environments next to the executable and their open tunnels.
func (a *App) ListEnvs() (EnvList, error) {
	names, err := env.List(a.dir)
	if err != nil {
		return EnvList{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	list := EnvList{Dir: a.dir, Active: a.settings.ActiveEnv, Envs: []EnvInfo{}}
	for _, name := range names {
		info := EnvInfo{Name: name, File: env.FileName(name)}
		for _, st := range a.states {
			if st.Env == name && st.Status == StatusConnected {
				info.ActiveTunnels++
			}
		}
		list.Envs = append(list.Envs, info)
	}
	return list, nil
}

// SelectEnv makes another environment active. Open tunnels keep running.
func (a *App) SelectEnv(name string) (EnvState, error) {
	names, err := env.List(a.dir)
	if err != nil {
		return EnvState{}, err
	}
	if !slices.Contains(names, name) {
		return EnvState{}, fmt.Errorf("o ambiente '%s' não existe", name)
	}
	a.mu.Lock()
	a.settings.ActiveEnv = name
	a.mu.Unlock()
	a.saveSettings()
	return a.GetEnv(), nil
}

// CreateEnv writes env.<name>.json and makes it active.
func (a *App) CreateEnv(req NewEnvRequest) (EnvState, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.RenameCurrentTo = strings.TrimSpace(req.RenameCurrentTo)
	if err := env.ValidateNewName(a.dir, req.Name); err != nil {
		return EnvState{}, err
	}

	current := a.activeName()
	created := &env.Env{Title: req.Name, Profiles: []env.Profile{}, Targets: []env.Target{}}
	if req.CopyCurrent {
		loaded, _, err := env.Load(filepath.Join(a.dir, env.FileName(current)))
		if err != nil {
			return EnvState{}, fmt.Errorf("não foi possível copiar o ambiente atual: %w", err)
		}
		created = loaded
	}

	// env.json disappears from the list once a named environment exists, so it is renamed first
	if _, err := os.Stat(filepath.Join(a.dir, env.DefaultFileName)); err == nil {
		if names, _ := env.List(a.dir); slices.Equal(names, []string{env.DefaultName}) {
			if req.RenameCurrentTo == "" {
				return EnvState{}, errors.New("dê um nome para o ambiente atual (env.json)")
			}
			if req.RenameCurrentTo == req.Name {
				return EnvState{}, errors.New("o ambiente atual e o novo precisam de nomes diferentes")
			}
			if a.openTunnels(env.DefaultName) > 0 {
				return EnvState{}, errors.New("desconecte os túneis de env.json antes de renomeá-lo")
			}
			if err := env.Rename(a.dir, env.DefaultName, req.RenameCurrentTo); err != nil {
				return EnvState{}, err
			}
			a.logf("env", fmt.Sprintf("env.json renomeado para %s.", env.FileName(req.RenameCurrentTo)))
		}
	}

	if err := env.Create(a.dir, req.Name, created); err != nil {
		return EnvState{}, err
	}
	a.logf("env", fmt.Sprintf("%s criado.", env.FileName(req.Name)))
	return a.SelectEnv(req.Name)
}

// GetEnv loads the active environment from disk.
func (a *App) GetEnv() EnvState {
	name := a.activeName()
	state := EnvState{Name: name, File: env.FileName(name), Path: filepath.Join(a.dir, env.FileName(name))}
	loaded, problems, err := env.Load(state.Path)
	var formatErr *jsonfile.FormatError
	switch {
	case errors.Is(err, env.ErrNotFound):
		state.Error = "notFound"
	case errors.As(err, &formatErr):
		state.Error = "invalidJson"
		state.ErrorDetail = formatErr.Error()
	case err != nil:
		state.Error = "read"
		state.ErrorDetail = err.Error()
	default:
		state.Env = loaded
		state.Problems = problems
	}

	a.mu.Lock()
	if state.Env != nil && len(state.Problems) == 0 {
		a.current = state.Env
	} else {
		a.current = nil
	}
	a.mu.Unlock()
	return state
}

// SaveEnv validates and writes the active environment. Open tunnels keep the
// settings they were started with.
func (a *App) SaveEnv(e env.Env) (SaveResult, error) {
	file := env.FileName(a.activeName())
	problems, err := env.Save(filepath.Join(a.dir, file), &e)
	if err != nil {
		return SaveResult{}, err
	}
	if len(problems) > 0 {
		return SaveResult{Problems: problems}, nil
	}
	a.logf("env", file+" salvo.")
	return SaveResult{State: a.GetEnv()}, nil
}

type Preferences struct {
	HideSensitive       bool `json:"hideSensitive"`
	OpenChromeOnConnect bool `json:"openChromeOnConnect"`
}

// GetPreferences returns the screen options stored in ssm-helper.json.
func (a *App) GetPreferences() Preferences {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Preferences{HideSensitive: a.settings.HideSensitive, OpenChromeOnConnect: a.settings.OpenChromeOnConnect}
}

// SetHideSensitive turns the privacy mode on or off.
func (a *App) SetHideSensitive(hide bool) {
	a.mu.Lock()
	a.settings.HideSensitive = hide
	a.mu.Unlock()
	a.saveSettings()
}

// SetOpenChromeOnConnect chooses whether Chrome opens when a tunnel is ready.
func (a *App) SetOpenChromeOnConnect(open bool) {
	a.mu.Lock()
	a.settings.OpenChromeOnConnect = open
	a.mu.Unlock()
	a.saveSettings()
}

// GetAppearance returns the appearance, which is the same for every environment.
func (a *App) GetAppearance() AppearanceState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return AppearanceState{Appearance: a.settings.Appearance, Builtins: settings.BuiltinPalettes()}
}

// SaveAppearance validates and stores the appearance in ssm-helper.json.
func (a *App) SaveAppearance(appearance settings.Appearance) ([]string, error) {
	problems, err := settings.ValidateAppearance(appearance)
	if err != nil || len(problems) > 0 {
		return problems, err
	}
	a.mu.Lock()
	previous := a.settings.Appearance
	a.settings.Appearance = appearance
	s := a.settings
	a.mu.Unlock()
	if err := settings.Save(a.dir, s); err != nil {
		a.mu.Lock()
		a.settings.Appearance = previous
		a.mu.Unlock()
		return nil, fmt.Errorf("não foi possível salvar %s (a pasta do programa permite escrita?): %w", settings.FileName, err)
	}
	a.logf("settings", "Aparência salva.")
	return nil, nil
}

// OpenAppFolder shows the folder of the program, which holds the environments.
func (a *App) OpenAppFolder() error {
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", a.dir).Start()
	}
	return exec.Command("xdg-open", a.dir).Start()
}

// ProfileStatuses reports the SSO session of each profile of the active environment.
func (a *App) ProfileStatuses() []ProfileStatus {
	current := a.currentEnv()
	if current == nil {
		return nil
	}
	statuses := make([]ProfileStatus, 0, len(current.Profiles))
	for _, p := range current.Profiles {
		st := a.sso.Status(a.ctx, p)
		ps := ProfileStatus{Name: p.Name, LoggedIn: st.LoggedIn}
		if st.LoggedIn {
			ps.ExpiresAt = st.ExpiresAt.Format(time.RFC3339)
		}
		statuses = append(statuses, ps)
	}
	return statuses
}

// Login runs the SSO login for a profile of the active environment and verifies the role with STS.
func (a *App) Login(profileName string) error {
	current := a.currentEnv()
	if current == nil {
		return errors.New("corrija o ambiente antes de fazer login")
	}
	p, ok := current.Profile(profileName)
	if !ok {
		return fmt.Errorf("perfil '%s' não existe", profileName)
	}
	ctx, done, ok := a.track(loginKey(a.activeName(), profileName))
	if !ok {
		return fmt.Errorf("já há um login em andamento para o perfil '%s'", profileName)
	}
	defer done()
	return a.login(ctx, p)
}

// CancelLogin stops a pending SSO login.
func (a *App) CancelLogin(profileName string) {
	a.cancel(loginKey(a.activeName(), profileName))
}

// TunnelStates returns the state of every target that was touched, in every environment.
func (a *App) TunnelStates() []TunnelState {
	a.mu.Lock()
	defer a.mu.Unlock()
	states := make([]TunnelState, 0, len(a.states))
	for _, st := range a.states {
		states = append(states, st)
	}
	return states
}

// Connect opens a tunnel for a target of the active environment in the
// background; progress arrives as tunnel:state events.
func (a *App) Connect(targetName string, openChrome bool) error {
	current := a.currentEnv()
	if current == nil {
		return errors.New("corrija o ambiente antes de conectar")
	}
	t, ok := current.Target(targetName)
	if !ok {
		return fmt.Errorf("target '%s' não existe", targetName)
	}
	p, _ := current.Profile(t.Profile)
	envName := a.activeName()

	ctx, done, ok := a.track(tunnelKey(envName, targetName))
	if !ok {
		return fmt.Errorf("o target '%s' já está conectado ou conectando", targetName)
	}
	go func() {
		defer done()
		a.connect(ctx, envName, p, t, openChrome)
	}()
	return nil
}

// Disconnect closes a tunnel (of any environment) or cancels a pending connection.
func (a *App) Disconnect(envName, targetName string) {
	key := tunnelKey(envName, targetName)
	a.cancel(key)
	a.mu.Lock()
	s := a.tunnels[key]
	a.mu.Unlock()
	if s != nil {
		s.Stop()
	}
}

// OpenChrome opens Chrome mapped to the tunnel of a target of the active environment.
func (a *App) OpenChrome(targetName string) error {
	current := a.currentEnv()
	if current == nil {
		return errors.New("corrija o ambiente antes de abrir o Chrome")
	}
	t, ok := current.Target(targetName)
	if !ok {
		return fmt.Errorf("target '%s' não existe", targetName)
	}
	return a.openChrome(t)
}

func (a *App) connect(ctx context.Context, envName string, p env.Profile, t env.Target, openChrome bool) {
	key := tunnelKey(envName, t.Name)
	set := func(status, message string) { a.setState(envName, t.Name, status, message) }
	set(StatusConnecting, "Verificando sessão AWS...")

	creds, err := a.sso.Credentials(ctx, p)
	if errors.Is(err, sso.ErrLoginRequired) {
		set(StatusLogin, "Sessão expirada. Conclua o login SSO no navegador.")
		if err = a.login(ctx, p); err == nil {
			creds, err = a.sso.Credentials(ctx, p)
		}
	}
	if err != nil {
		a.fail(ctx, envName, t.Name, err)
		return
	}

	set(StatusConnecting, "Iniciando túnel SSM...")
	s, err := tunnel.Start(ctx, creds, p, t, func(line string) { a.logf(t.Name, line) })
	if err != nil {
		a.fail(ctx, envName, t.Name, err)
		return
	}
	a.mu.Lock()
	a.tunnels[key] = s
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.tunnels, key)
		a.mu.Unlock()
	}()

	select {
	case <-s.Ready():
		set(StatusConnected, fmt.Sprintf("Túnel ativo em localhost:%s", t.LocalPort))
		if openChrome && t.UsesBrowser() {
			if err := a.openChrome(t); err != nil {
				a.logf(t.Name, err.Error())
			}
		}
	case <-s.Done():
		// The plugin exits with code 0 on most failures, so report its last message
		if s.Err() == nil && ctx.Err() == nil {
			set(StatusError, "O túnel foi encerrado antes de ficar pronto: "+s.LastOutput())
			return
		}
	case <-ctx.Done():
		s.Stop()
	}

	select {
	case <-s.Done():
	case <-ctx.Done():
		s.Stop()
	}
	if err := s.Err(); err != nil {
		set(StatusError, fmt.Sprintf("O túnel foi encerrado com erro: %v", err))
		return
	}
	set(StatusIdle, "Túnel encerrado.")
}

func (a *App) login(ctx context.Context, p env.Profile) error {
	a.logf(p.Name, "Iniciando login SSO. O navegador será aberto.")
	err := a.sso.Login(ctx, p, func(code sso.DeviceCode) {
		wruntime.EventsEmit(a.ctx, eventSSOCode, SSOCode{Profile: p.Name, Code: code})
	})
	wruntime.EventsEmit(a.ctx, eventSSOCode, SSOCode{Profile: p.Name})
	if err != nil {
		return err
	}
	if err := a.sso.Verify(ctx, p); err != nil {
		return fmt.Errorf("%w. Confirme com o TechLead se você possui a role '%s'", err, p.RoleName)
	}
	a.logf(p.Name, fmt.Sprintf("Perfil '%s' autenticado via SSO com sucesso!", p.Name))
	return nil
}

func (a *App) openChrome(t env.Target) error {
	commandLine, err := chrome.Launch(t, a.dataDir)
	if err != nil {
		if commandLine != "" {
			a.logf(t.Name, "Comando do Chrome: "+commandLine)
		}
		return err
	}
	a.logf(t.Name, "Chrome aberto apontando para o túnel.")
	return nil
}

func (a *App) fail(ctx context.Context, envName, target string, err error) {
	if ctx.Err() != nil {
		a.setState(envName, target, StatusIdle, "Conexão cancelada.")
		return
	}
	a.logf(target, err.Error())
	a.setState(envName, target, StatusError, err.Error())
}

func (a *App) currentEnv() *env.Env {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current
}

func (a *App) activeName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.ActiveEnv
}

func (a *App) openTunnels(envName string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for key := range a.tunnels {
		if strings.HasPrefix(key, envName+"/") {
			n++
		}
	}
	return n
}

// saveSettings stores the selected environment; failures are only logged, the app keeps working.
func (a *App) saveSettings() {
	a.mu.Lock()
	s := a.settings
	a.mu.Unlock()
	if err := settings.Save(a.dir, s); err != nil {
		a.logf("settings", fmt.Sprintf("Não foi possível salvar %s (a pasta do programa permite escrita?): %v", settings.FileName, err))
	}
}

// tunnelKey identifies a target across environments; names cannot contain "/".
func tunnelKey(envName, target string) string { return envName + "/" + target }

func loginKey(envName, profile string) string { return "profile:" + envName + "/" + profile }

// track registers a cancelable operation under key; done must be called when
// it ends. It fails when an operation with the same key is already running.
func (a *App) track(key string) (context.Context, func(), bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, busy := a.cancels[key]; busy {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancels[key] = cancel
	return ctx, func() {
		cancel()
		a.mu.Lock()
		delete(a.cancels, key)
		a.mu.Unlock()
	}, true
}

func (a *App) cancel(key string) {
	a.mu.Lock()
	cancel := a.cancels[key]
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) setState(envName, target, status, message string) {
	st := TunnelState{Env: envName, Target: target, Status: status, Message: message}
	a.mu.Lock()
	a.states[tunnelKey(envName, target)] = st
	a.mu.Unlock()
	wruntime.EventsEmit(a.ctx, eventTunnelState, st)
}

func (a *App) logf(source, text string) {
	if a.ctx == nil {
		a.note(source, text)
		return
	}
	wruntime.EventsEmit(a.ctx, eventLog, newLogEntry(source, text))
}

// note keeps a message for StartupMessages.
func (a *App) note(source, text string) {
	a.mu.Lock()
	a.startup = append(a.startup, newLogEntry(source, text))
	a.mu.Unlock()
}

func newLogEntry(source, text string) LogEntry {
	return LogEntry{Time: time.Now().Format("15:04:05"), Source: source, Text: text}
}

// startupBackground is the window color before the frontend loads, taken from
// the active palette.
func (a *App) startupBackground() *options.RGBA {
	a.mu.Lock()
	palette := a.settings.Appearance.ActivePalette()
	a.mu.Unlock()
	var r, g, b uint8
	if _, err := fmt.Sscanf(palette.Colors["bgDeep"], "#%02x%02x%02x", &r, &g, &b); err != nil {
		r, g, b = 0x14, 0x17, 0x1c
	}
	return &options.RGBA{R: r, G: g, B: b, A: 255}
}

// dataDirName holds what the app generates (SSO cache, Chrome profiles,
// WebView2 data) next to the executable, so the whole app travels with its folder.
const dataDirName = "ssm-helper-data"

// defaultPaths returns the folder of the executable (or SSM_HELPER_DIR), which
// holds the environments and ssm-helper.json, and its data folder.
func defaultPaths() (dir, dataDir string, err error) {
	dir = os.Getenv("SSM_HELPER_DIR")
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", "", err
		}
		dir = filepath.Dir(exe)
	}
	return dir, filepath.Join(dir, dataDirName), nil
}

// moveLegacyDataDir moves %LOCALAPPDATA%\ssm-helper, used by earlier versions,
// into the data folder, so the SSO login and the Chrome profiles are kept.
func (a *App) moveLegacyDataDir() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	legacy := filepath.Join(cacheDir, "ssm-helper")
	if _, err := os.Stat(a.dataDir); err == nil {
		return
	}
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		return
	}
	if err := os.Rename(legacy, a.dataDir); err == nil {
		a.note("data", fmt.Sprintf("Login SSO e perfis do Chrome movidos de %s para %s.", legacy, a.dataDir))
		return
	}
	// Another drive: copy the SSO login, which is small; Chrome profiles start fresh
	if err := copyDir(filepath.Join(legacy, "sso-cache"), filepath.Join(a.dataDir, "sso-cache")); err != nil {
		a.note("data", fmt.Sprintf("Não foi possível trazer os dados de %s: %v. Faça login de novo se for pedido.", legacy, err))
		return
	}
	a.note("data", fmt.Sprintf("Login SSO copiado de %s. Os perfis do Chrome começam vazios; a pasta antiga pode ser apagada.", legacy))
}

func copyDir(from, to string) error {
	entries, err := os.ReadDir(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
