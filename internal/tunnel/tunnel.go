// Package tunnel starts SSM port forwarding sessions. The Session Manager
// plugin is compiled into this executable, but it calls os.Exit when a session
// ends, so each session runs in a child process of the same executable.
package tunnel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	_ "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session/portsession"

	"github.com/v-felipeolliveira/ssm-helper/internal/env"
	"github.com/v-felipeolliveira/ssm-helper/internal/msg"
	"github.com/v-felipeolliveira/ssm-helper/internal/sso"
)

// PluginArg is the first argument that makes the executable run as the plugin.
const PluginArg = "--ssm-session-plugin"

// responseEnv holds the StartSession response, so it is not visible in the command line.
const responseEnv = "AWS_SSM_START_SESSION_RESPONSE_SSM_HELPER"

// readyMessage is printed by the plugin once the local port is listening.
const readyMessage = "Waiting for connections"

// RunPlugin runs the Session Manager plugin with the arguments after PluginArg.
func RunPlugin(args []string) {
	session.ValidateInputAndStartSession(args, os.Stdout)
}

// Session is a running port forwarding tunnel.
type Session struct {
	Target    env.Target
	SessionID string

	cmd       *exec.Cmd
	client    *ssm.Client
	ready     chan struct{}
	done      chan struct{}
	err       error
	readyOnce sync.Once
	stopOnce  sync.Once
	stopping  atomic.Bool
	lastLine  atomic.Value // string
}

// Start opens the tunnel for the target. logf receives the plugin output.
func Start(ctx context.Context, creds aws.Credentials, p env.Profile, t env.Target, logf func(string)) (*Session, error) {
	if err := checkPortFree(t.LocalPort); err != nil {
		return nil, err
	}

	region := t.Region(p)
	// The SDK builds the endpoint host from the region without checking it
	if !env.ValidRegion(region) {
		return nil, msg.Err(msg.RegionInvalid, "region", region)
	}
	client := ssm.NewFromConfig(sso.AWSConfig(creds, region))
	out, err := client.StartSession(ctx, &ssm.StartSessionInput{
		Target:       aws.String(t.SSMTarget),
		DocumentName: aws.String(t.SSMDocument),
		Parameters: map[string][]string{
			"host":            {t.RemoteHost},
			"portNumber":      {t.RemotePort},
			"localPortNumber": {t.LocalPort},
		},
	})
	if err != nil {
		return nil, msg.Wrap(msg.TunnelStartFailed, err)
	}

	response, err := json.Marshal(map[string]string{
		"SessionId":  aws.ToString(out.SessionId),
		"StreamUrl":  aws.ToString(out.StreamUrl),
		"TokenValue": aws.ToString(out.TokenValue),
	})
	if err != nil {
		return nil, err
	}
	request, err := json.Marshal(map[string]string{"Target": t.SSMTarget})
	if err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	// Same argument layout the AWS CLI uses to call session-manager-plugin
	cmd := exec.Command(exe, PluginArg,
		responseEnv,
		region,
		"StartSession",
		"", // profile: credentials come from the environment
		string(request),
		fmt.Sprintf("https://ssm.%s.amazonaws.com", region),
	)
	cmd.Env = pluginEnv(creds, region, string(response))
	configureCommand(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	s := &Session{
		Target:    t,
		SessionID: aws.ToString(out.SessionId),
		cmd:       cmd,
		client:    client,
		ready:     make(chan struct{}),
		done:      make(chan struct{}),
	}
	if err := cmd.Start(); err != nil {
		s.terminate()
		return nil, msg.Wrap(msg.TunnelProcessFailed, err)
	}
	attachToJob(cmd)

	go s.readOutput(stdout, logf)
	go func() {
		err := cmd.Wait()
		if err != nil && !s.stopping.Load() {
			s.err = err
		}
		s.terminate()
		close(s.done)
	}()
	return s, nil
}

// Ready is closed once the local port accepts connections.
func (s *Session) Ready() <-chan struct{} { return s.ready }

// Done is closed when the tunnel process exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns the process error after Done, or nil when it ended normally.
func (s *Session) Err() error { return s.err }

// LastOutput returns the last line printed by the plugin.
func (s *Session) LastOutput() string {
	line, _ := s.lastLine.Load().(string)
	return line
}

// Stop ends the tunnel and waits for the process to exit.
func (s *Session) Stop() {
	s.stopOnce.Do(func() {
		s.stopping.Store(true)
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	})
	<-s.done
}

// terminate tells SSM the session is over, so it does not wait for the idle timeout.
func (s *Session) terminate() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = s.client.TerminateSession(ctx, &ssm.TerminateSessionInput{SessionId: aws.String(s.SessionID)})
}

func (s *Session) readOutput(r io.Reader, logf func(string)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		s.lastLine.Store(line)
		if strings.Contains(line, readyMessage) {
			s.readyOnce.Do(func() { close(s.ready) })
		}
		if logf != nil {
			logf(line)
		}
	}
}

func pluginEnv(creds aws.Credentials, region, response string) []string {
	env := []string{
		"AWS_ACCESS_KEY_ID=" + creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY=" + creds.SecretAccessKey,
		"AWS_SESSION_TOKEN=" + creds.SessionToken,
		"AWS_REGION=" + region,
		// Ignore the user's ~/.aws files, like the script did
		"AWS_CONFIG_FILE=" + os.DevNull,
		"AWS_SHARED_CREDENTIALS_FILE=" + os.DevNull,
		responseEnv + "=" + response,
	}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToUpper(name), "AWS_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

// checkPortFree fails when another process already listens on the local port.
func checkPortFree(port string) error {
	l, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		return msg.Wrap(msg.TunnelPortInUse, errors.Unwrap(err), "port", port)
	}
	return l.Close()
}
