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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipeolliveira/ssm-helper/internal/applog"
	"github.com/felipeolliveira/ssm-helper/internal/chrome"
	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
	"github.com/felipeolliveira/ssm-helper/internal/settings"
	"github.com/felipeolliveira/ssm-helper/internal/sso"
	"github.com/felipeolliveira/ssm-helper/internal/tunnel"
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
	Name     string        `json:"name"`
	File     string        `json:"file"`
	Path     string        `json:"path"`
	Env      *env.Env      `json:"env"`
	Problems []msg.Message `json:"problems"`
	// Error is "notFound", "invalidJson" or "read" when the file could not be loaded.
	Error       string `json:"error"`
	ErrorDetail string `json:"errorDetail"`
}

// SaveResult is the outcome of SaveEnv.
type SaveResult struct {
	Problems []msg.Message `json:"problems"`
	State    EnvState      `json:"state"`
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
	Env     string      `json:"env"`
	Target  string      `json:"target"`
	Status  string      `json:"status"`
	Message msg.Message `json:"message"`
}

type ProfileStatus struct {
	Name      string `json:"name"`
	LoggedIn  bool   `json:"loggedIn"`
	ExpiresAt string `json:"expiresAt"`
}

type LogEntry struct {
	Time    string      `json:"time"`
	Source  string      `json:"source"`
	Message msg.Message `json:"message"`
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
	logFile *applog.File // ssm-helper-data/logs, one file per day

	mu       sync.Mutex
	settings settings.Settings
	current  *env.Env // active environment, only when valid
	tunnels  map[string]*tunnel.Session
	states   map[string]TunnelState
	cancels  map[string]context.CancelFunc // pending connects and logins
	startup  []LogEntry                    // messages from before the window opened
	logError bool                          // a log file write failed and was reported
}

func NewApp(dir, dataDir string) *App {
	a := &App{
		dir:     dir,
		dataDir: dataDir,
		tunnels: map[string]*tunnel.Session{},
		states:  map[string]TunnelState{},
		cancels: map[string]context.CancelFunc{},
		logFile: applog.New(filepath.Join(dataDir, logDirName)),
	}
	a.sso = sso.NewManager(filepath.Join(dataDir, "sso-cache"), func(url string) {
		wruntime.BrowserOpenURL(a.ctx, url)
	})
	// Checked before secureDataDir creates the data folder
	legacy := a.legacyDataDir()
	a.secureDataDir()
	if legacy != "" {
		a.note("data", msg.New(msg.DataLegacyDirUnused, "legacy", legacy, "dir", a.dataDir))
	}
	a.loadSettings()
	return a
}

// loadSettings reads ssm-helper.json and migrates files from older versions:
// config.yaml/config.json become env files, and ssm-helper.settings.json becomes ssm-helper.json.
func (a *App) loadSettings() {
	s, problems, source, err := settings.Load(a.dir)
	if err != nil {
		a.note("settings", msg.From(msg.Wrap(msg.SettingsReadFailed, err, "file", settings.FileName)))
	}
	for _, p := range problems {
		a.note("settings", p)
	}
	if len(problems) > 0 {
		a.note("settings", msg.New(msg.SettingsDefaultsUsed, "file", settings.FileName))
	}
	save := source == settings.SourceLegacyFile
	if save {
		a.note("settings", msg.New(msg.SettingsMigrated, "from", settings.LegacyFileName, "to", settings.FileName))
	}

	migration, err := env.MigrateLegacy(a.dir)
	if err != nil {
		a.note("env", msg.From(err))
	}
	froms := make([]string, 0, len(migration.Files))
	for from := range migration.Files {
		froms = append(froms, from)
	}
	slices.Sort(froms)
	for _, from := range froms {
		a.note("env", msg.New(msg.EnvMigrated, "from", from, "to", migration.Files[from]))
		save = true
	}
	// The appearance used to live in config.yaml
	if source == settings.SourceNone && len(migration.AppearanceSources) > 0 {
		from := migration.AppearanceSources[0]
		if data, err := os.ReadFile(filepath.Join(a.dir, from)); err == nil {
			if appearance, problems := settings.ParseAppearance(env.AppearanceNode(data)); len(problems) == 0 {
				s.Appearance = appearance
				a.note("settings", msg.New(msg.AppearanceMigrated, "from", from, "to", settings.FileName))
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
	a.logFile.Close()
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
		return EnvState{}, msg.Err(msg.EnvDoesNotExist, "name", name)
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
			return EnvState{}, msg.Wrap(msg.EnvCopyFailed, err)
		}
		created = loaded
	}

	// env.json disappears from the list once a named environment exists, so it is renamed first
	if _, err := os.Stat(filepath.Join(a.dir, env.DefaultFileName)); err == nil {
		if names, _ := env.List(a.dir); slices.Equal(names, []string{env.DefaultName}) {
			if req.RenameCurrentTo == "" {
				return EnvState{}, msg.Err(msg.EnvRenameCurrentRequired)
			}
			if req.RenameCurrentTo == req.Name {
				return EnvState{}, msg.Err(msg.EnvRenameSameName)
			}
			if a.openTunnels(env.DefaultName) > 0 {
				return EnvState{}, msg.Err(msg.EnvRenameHasTunnels)
			}
			if err := env.Rename(a.dir, env.DefaultName, req.RenameCurrentTo); err != nil {
				return EnvState{}, err
			}
			a.logf("env", msg.New(msg.EnvRenamed, "from", env.DefaultFileName, "to", env.FileName(req.RenameCurrentTo)))
		}
	}

	if err := env.Create(a.dir, req.Name, created); err != nil {
		return EnvState{}, err
	}
	a.logf("env", msg.New(msg.EnvCreated, "file", env.FileName(req.Name)))
	return a.SelectEnv(req.Name)
}

// GetEnv loads the active environment from disk.
func (a *App) GetEnv() EnvState {
	name := a.activeName()
	state := EnvState{Name: name, File: env.FileName(name), Path: filepath.Join(a.dir, env.FileName(name))}
	loaded, problems, err := env.Load(state.Path)
	switch {
	case errors.Is(err, env.ErrNotFound):
		state.Error = "notFound"
	case jsonfile.IsFormatError(err):
		state.Error = "invalidJson"
		state.ErrorDetail = msg.From(err).Detail
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
	a.logf("env", msg.New(msg.EnvSaved, "file", file))
	return SaveResult{State: a.GetEnv()}, nil
}

type Preferences struct {
	// Language is "pt-BR", "en" or empty (follow Windows, decided by the frontend).
	Language            string `json:"language"`
	HideSensitive       bool   `json:"hideSensitive"`
	OpenChromeOnConnect bool   `json:"openChromeOnConnect"`
}

// GetPreferences returns the screen options stored in ssm-helper.json.
func (a *App) GetPreferences() Preferences {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Preferences{
		Language:            a.settings.Language,
		HideSensitive:       a.settings.HideSensitive,
		OpenChromeOnConnect: a.settings.OpenChromeOnConnect,
	}
}

// SetLanguage stores the language of the screens ("pt-BR" or "en").
func (a *App) SetLanguage(lang string) {
	a.mu.Lock()
	a.settings.Language = settings.ValidLanguage(lang)
	a.mu.Unlock()
	a.saveSettings()
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
func (a *App) SaveAppearance(appearance settings.Appearance) ([]msg.Message, error) {
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
		return nil, msg.Wrap(msg.SettingsSaveFailed, err, "file", settings.FileName)
	}
	a.logf("settings", msg.New(msg.AppearanceSaved))
	return nil, nil
}

// OpenAppFolder shows the folder of the program, which holds the environments.
func (a *App) OpenAppFolder() error {
	return openFolder(a.dir)
}

// OpenLogFolder shows the folder with the log files, creating it if needed.
func (a *App) OpenLogFolder() error {
	if err := os.MkdirAll(a.logFile.Dir, 0o700); err != nil {
		return msg.Wrap(msg.LogDirOpenFailed, err, "dir", a.logFile.Dir)
	}
	if err := openFolder(a.logFile.Dir); err != nil {
		return msg.Wrap(msg.LogDirOpenFailed, err, "dir", a.logFile.Dir)
	}
	return nil
}

func openFolder(dir string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", dir).Start()
	}
	return exec.Command("xdg-open", dir).Start()
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
		return msg.Err(msg.EnvNotReady)
	}
	p, ok := current.Profile(profileName)
	if !ok {
		return msg.Err(msg.ProfileNotFound, "profile", profileName)
	}
	ctx, done, ok := a.track(loginKey(a.activeName(), profileName))
	if !ok {
		return msg.Err(msg.SSOLoginInProgress, "profile", profileName)
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
		return msg.Err(msg.EnvNotReady)
	}
	t, ok := current.Target(targetName)
	if !ok {
		return msg.Err(msg.TargetNotFound, "target", targetName)
	}
	p, _ := current.Profile(t.Profile)
	envName := a.activeName()

	ctx, done, ok := a.track(tunnelKey(envName, targetName))
	if !ok {
		return msg.Err(msg.TunnelAlreadyActive, "target", targetName)
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
		return msg.Err(msg.EnvNotReady)
	}
	t, ok := current.Target(targetName)
	if !ok {
		return msg.Err(msg.TargetNotFound, "target", targetName)
	}
	return a.openChrome(t)
}

// Reconnection of a tunnel that drops by itself (SSM idle timeout, network
// change, laptop sleep): a new SSM session with fresh credentials, after a
// growing delay. The count of failures starts again once a tunnel stays up
// for reconnectStableAfter.
const (
	reconnectMaxFailures = 10
	reconnectStableAfter = time.Minute
)

var reconnectDelays = []time.Duration{
	2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second, time.Minute,
}

func reconnectDelay(failure int) time.Duration {
	return reconnectDelays[min(failure, len(reconnectDelays))-1]
}

// sessionResult is how one SSM session of a tunnel ended.
type sessionResult struct {
	ready     bool          // the local port was listening
	connected time.Duration // time since it was ready
	reason    msg.Message   // why it ended, when not asked by the user
}

// connect keeps the tunnel of a target open until ctx is cancelled (the user
// disconnects or the app closes), reconnecting when it drops.
func (a *App) connect(ctx context.Context, envName string, p env.Profile, t env.Target, openChrome bool) {
	set := func(status string, m msg.Message) { a.setState(envName, t.Name, status, m) }
	set(StatusConnecting, msg.New(msg.TunnelCheckingSession))

	creds, err := a.sso.Credentials(ctx, p)
	if errors.Is(err, sso.ErrLoginRequired) {
		set(StatusLogin, msg.New(msg.TunnelLoginRequired))
		if err = a.login(ctx, p); err == nil {
			creds, err = a.sso.Credentials(ctx, p)
		}
	}
	if err != nil {
		a.fail(ctx, envName, t.Name, err)
		return
	}

	set(StatusConnecting, msg.New(msg.TunnelStarting))
	res := a.runSession(ctx, envName, creds, p, t, func() {
		if openChrome && t.UsesBrowser() {
			if err := a.openChrome(t); err != nil {
				a.logf(t.Name, msg.From(err))
			}
		}
	})
	if ctx.Err() != nil {
		set(StatusIdle, msg.New(msg.TunnelClosed))
		return
	}
	if !res.ready {
		// A first connection that fails is a setup problem, not a drop
		a.logf(t.Name, res.reason)
		set(StatusError, res.reason)
		return
	}
	failures := 0
	for {
		if res.ready {
			a.logf(t.Name, msg.Message{
				Code:   msg.TunnelDropped,
				Params: map[string]string{"after": formatDuration(res.connected)},
				Detail: res.reason.Detail,
			})
			if res.connected >= reconnectStableAfter {
				failures = 0
			}
		} else {
			a.logf(t.Name, res.reason)
		}
		failures++
		if failures > reconnectMaxFailures {
			m := msg.Message{
				Code:   msg.TunnelReconnectGaveUp,
				Params: map[string]string{"attempts": fmt.Sprint(reconnectMaxFailures)},
				Detail: res.reason.Detail,
			}
			a.logf(t.Name, m)
			set(StatusError, m)
			return
		}

		delay := reconnectDelay(failures)
		attempt := map[string]string{"attempt": fmt.Sprint(failures), "max": fmt.Sprint(reconnectMaxFailures)}
		waiting := msg.Message{Code: msg.TunnelReconnectWaiting, Params: map[string]string{"seconds": fmt.Sprint(int(delay.Seconds()))}}
		for k, v := range attempt {
			waiting.Params[k] = v
		}
		a.logf(t.Name, waiting)
		set(StatusConnecting, waiting)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			set(StatusIdle, msg.New(msg.TunnelClosed))
			return
		}

		set(StatusConnecting, msg.Message{Code: msg.TunnelReconnecting, Params: attempt})
		// No SSO login without the user: it would open the browser on its own
		creds, err := a.sso.Credentials(ctx, p)
		if errors.Is(err, sso.ErrLoginRequired) {
			m := msg.New(msg.TunnelReconnectLoginRequired)
			a.logf(t.Name, m)
			set(StatusError, m)
			return
		}
		if err != nil {
			res = sessionResult{reason: msg.From(err)}
		} else {
			// Chrome is not opened again: it keeps working on the same port
			res = a.runSession(ctx, envName, creds, p, t, func() {
				a.logf(t.Name, msg.New(msg.TunnelReconnected, "port", t.LocalPort))
			})
		}
		if ctx.Err() != nil {
			set(StatusIdle, msg.New(msg.TunnelClosed))
			return
		}
	}
}

// runSession runs one SSM session of the tunnel until it ends or ctx is
// cancelled. It reports the tunnel as connected while it is ready and calls
// onReady once the local port listens.
func (a *App) runSession(ctx context.Context, envName string, creds aws.Credentials, p env.Profile, t env.Target, onReady func()) sessionResult {
	key := tunnelKey(envName, t.Name)
	s, err := tunnel.Start(ctx, creds, p, t, func(line string) {
		a.logf(t.Name, msg.Message{Code: msg.TunnelPluginOutput, Detail: line})
	})
	if err != nil {
		return sessionResult{reason: msg.From(err)}
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
	case <-s.Done():
		return sessionResult{reason: endReason(s, msg.TunnelEndedBeforeReady)}
	case <-ctx.Done():
		s.Stop()
		return sessionResult{}
	}

	readyAt := time.Now()
	a.setState(envName, t.Name, StatusConnected, msg.New(msg.TunnelActive, "port", t.LocalPort))
	onReady()
	select {
	case <-s.Done():
	case <-ctx.Done():
		s.Stop()
	}
	return sessionResult{ready: true, connected: time.Since(readyAt), reason: endReason(s, msg.TunnelEndedWithError)}
}

// endReason describes how the plugin ended. It exits with code 0 on most
// failures, so its last line is the best explanation.
func endReason(s *tunnel.Session, code string) msg.Message {
	if err := s.Err(); err != nil {
		return msg.From(msg.Wrap(code, err))
	}
	return msg.Message{Code: code, Detail: s.LastOutput()}
}

func formatDuration(d time.Duration) string {
	return d.Round(time.Second).String()
}

func (a *App) login(ctx context.Context, p env.Profile) error {
	a.logf(p.Name, msg.New(msg.SSOLoginStarting))
	err := a.sso.Login(ctx, p, func(code sso.DeviceCode) {
		wruntime.EventsEmit(a.ctx, eventSSOCode, SSOCode{Profile: p.Name, Code: code})
	})
	wruntime.EventsEmit(a.ctx, eventSSOCode, SSOCode{Profile: p.Name})
	if err != nil {
		return err
	}
	if err := a.sso.Verify(ctx, p); err != nil {
		return err
	}
	a.logf(p.Name, msg.New(msg.SSOLoginSucceeded, "profile", p.Name))
	return nil
}

func (a *App) openChrome(t env.Target) error {
	commandLine, err := chrome.Launch(t, a.dataDir)
	if err != nil {
		if commandLine != "" {
			a.logf(t.Name, msg.Message{Code: msg.ChromeCommand, Detail: commandLine})
		}
		return err
	}
	a.logf(t.Name, msg.New(msg.ChromeOpened))
	return nil
}

func (a *App) fail(ctx context.Context, envName, target string, err error) {
	if ctx.Err() != nil {
		a.setState(envName, target, StatusIdle, msg.New(msg.TunnelCancelled))
		return
	}
	a.logf(target, msg.From(err))
	a.setState(envName, target, StatusError, msg.From(err))
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
		a.logf("settings", msg.From(msg.Wrap(msg.SettingsSaveFailed, err, "file", settings.FileName)))
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

func (a *App) setState(envName, target, status string, m msg.Message) {
	st := TunnelState{Env: envName, Target: target, Status: status, Message: m}
	a.mu.Lock()
	a.states[tunnelKey(envName, target)] = st
	a.mu.Unlock()
	wruntime.EventsEmit(a.ctx, eventTunnelState, st)
}

// logf shows a message in the log panel and writes it to the log file.
func (a *App) logf(source string, m msg.Message) {
	if a.ctx == nil {
		a.note(source, m)
		return
	}
	now := time.Now()
	a.writeLogFile(now, source, m)
	a.show(newLogEntry(now, source, m))
}

// note keeps a message for StartupMessages and writes it to the log file.
func (a *App) note(source string, m msg.Message) {
	now := time.Now()
	a.writeLogFile(now, source, m)
	a.mu.Lock()
	a.startup = append(a.startup, newLogEntry(now, source, m))
	a.mu.Unlock()
}

// show sends an entry to the log panel, or keeps it for StartupMessages
// before the window opens.
func (a *App) show(entry LogEntry) {
	if a.ctx == nil {
		a.mu.Lock()
		a.startup = append(a.startup, entry)
		a.mu.Unlock()
		return
	}
	wruntime.EventsEmit(a.ctx, eventLog, entry)
}

// writeLogFile appends to the log file of the day. Only the first failure is
// shown, so a full disk does not flood the panel; the app keeps working.
func (a *App) writeLogFile(t time.Time, source string, m msg.Message) {
	err := a.logFile.Write(t, source, m)
	a.mu.Lock()
	report := err != nil && !a.logError
	a.logError = err != nil
	a.mu.Unlock()
	if report {
		a.show(newLogEntry(t, "log", msg.From(msg.Wrap(msg.LogFileWriteFailed, err, "dir", a.logFile.Dir))))
	}
}

func newLogEntry(t time.Time, source string, m msg.Message) LogEntry {
	return LogEntry{Time: t.Format("15:04:05"), Source: source, Message: m}
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

// logDirName is the folder of the daily log files, inside the data folder.
const logDirName = "logs"

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

// secureDataDir creates the data folder and limits it to the current user.
func (a *App) secureDataDir() {
	if err := os.MkdirAll(a.dataDir, 0o700); err != nil {
		a.note("data", msg.From(msg.Wrap(msg.DataDirCreateFailed, err, "dir", a.dataDir)))
		return
	}
	if err := protectDir(a.dataDir); err != nil {
		a.note("data", msg.From(msg.Wrap(msg.DataDirNotProtected, err, "dir", a.dataDir)))
	}
}

// legacyDataDir returns %LOCALAPPDATA%\ssm-helper, used by earlier versions,
// when it exists and the data folder does not yet. It is not moved: any copy
// of the app would take the SSO login and the Chrome profiles from the others.
func (a *App) legacyDataDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	legacy := filepath.Join(cacheDir, "ssm-helper")
	if _, err := os.Stat(a.dataDir); err == nil {
		return ""
	}
	if info, err := os.Stat(legacy); err == nil && info.IsDir() {
		return legacy
	}
	return ""
}
