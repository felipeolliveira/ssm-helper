package env

import (
	"gopkg.in/yaml.v3"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/v-felipeolliveira/ssm-helper/internal/msg"
)

// Marshal writes the environment as JSON, keeping the order of profiles and targets.
func Marshal(e *Env) ([]byte, error) {
	profiles := &yaml.Node{Kind: yaml.MappingNode}
	for _, p := range e.Profiles {
		profiles.Content = append(profiles.Content, jsonfile.Key(p.Name), jsonfile.Strings(
			"ssoUrl", p.SSOURL,
			"ssoRegion", p.SSORegion,
			"accountId", p.AccountID,
			"roleName", p.RoleName,
		))
	}

	targets := &yaml.Node{Kind: yaml.MappingNode}
	for _, t := range e.Targets {
		node := jsonfile.Strings(
			"profile", t.Profile,
			"ssmTarget", t.SSMTarget,
			"ssmRegion", t.SSMRegion,
			"ssmDocument", t.SSMDocument,
			"remoteHost", t.RemoteHost,
			"remotePort", t.RemotePort,
			"localPort", t.LocalPort,
		)
		node.Content = append(node.Content,
			jsonfile.Key("dnsHosts"), jsonfile.StringList(t.DNSHosts),
			jsonfile.Key("openUrls"), jsonfile.StringList(t.OpenURLs),
		)
		node.Content = append(node.Content, jsonfile.Strings("chromeProfile", t.ChromeProfile).Content...)
		targets.Content = append(targets.Content, jsonfile.Key(t.Name), node)
	}

	root := jsonfile.Strings("title", e.Title)
	root.Content = append(root.Content,
		jsonfile.Key("profiles"), profiles,
		jsonfile.Key("targets"), targets,
	)
	return jsonfile.Encode(root)
}

// Save validates and writes the environment to path. When there are problems
// the file is left untouched and the problems are returned. The previous file
// is kept as <path>.bak.
func Save(path string, e *Env) ([]msg.Message, error) {
	data, err := Marshal(e)
	if err != nil {
		return nil, err
	}
	if _, problems, err := Parse(path, data); err != nil {
		return nil, err
	} else if len(problems) > 0 {
		return problems, nil
	}
	return nil, jsonfile.WriteAtomic(path, data, true)
}
