/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provision

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"text/template"
	"time"

	"github.com/golang/glog"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/cert"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/engine"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/provision"
	"github.com/docker/machine/libmachine/provision/pkgaction"
	"github.com/docker/machine/libmachine/provision/serviceaction"
	"github.com/docker/machine/libmachine/swarm"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/console"
	"k8s.io/minikube/pkg/minikube/rexec"
	"k8s.io/minikube/pkg/minikube/sshutil"
	"k8s.io/minikube/pkg/util"
)

type BuildrootProvisioner struct {
	provision.SystemdProvisioner
}

func init() {
	provision.Register("Buildroot", &provision.RegisteredProvisioner{
		New: NewBuildrootProvisioner,
	})
}

func NewBuildrootProvisioner(d drivers.Driver) provision.Provisioner {
	return &BuildrootProvisioner{
		provision.NewSystemdProvisioner("buildroot", d),
	}
}

func (p *BuildrootProvisioner) String() string {
	return "buildroot"
}

func (p *BuildrootProvisioner) GenerateDockerOptions(dockerPort int) (*provision.DockerOptions, error) {
	var engineCfg bytes.Buffer

	driverNameLabel := fmt.Sprintf("provider=%s", p.Driver.DriverName())
	p.EngineOptions.Labels = append(p.EngineOptions.Labels, driverNameLabel)

	engineConfigTmpl := `[Unit]
Description=Docker Application Container Engine
Documentation=https://docs.docker.com
After=network.target  minikube-automount.service docker.socket
Requires= minikube-automount.service docker.socket 

[Service]
Type=notify

# DOCKER_RAMDISK disables pivot_root in Docker, using MS_MOVE instead.
Environment=DOCKER_RAMDISK=yes
{{range .EngineOptions.Env}}Environment={{.}}
{{end}}

# This file is a systemd drop-in unit that inherits from the base dockerd configuration.
# The base configuration already specifies an 'ExecStart=...' command. The first directive
# here is to clear out that command inherited from the base configuration. Without this,
# the command from the base configuration and the command specified here are treated as
# a sequence of commands, which is not the desired behavior, nor is it valid -- systemd
# will catch this invalid input and refuse to start the service with an error like:
#  Service has more than one ExecStart= setting, which is only allowed for Type=oneshot services.
ExecStart=
ExecStart=/usr/bin/dockerd -H tcp://0.0.0.0:{{.DockerPort}} -H unix:///var/run/docker.sock --tlsverify --tlscacert {{.AuthOptions.CaCertRemotePath}} --tlscert {{.AuthOptions.ServerCertRemotePath}} --tlskey {{.AuthOptions.ServerKeyRemotePath}} {{ range .EngineOptions.Labels }}--label {{.}} {{ end }}{{ range .EngineOptions.InsecureRegistry }}--insecure-registry {{.}} {{ end }}{{ range .EngineOptions.RegistryMirror }}--registry-mirror {{.}} {{ end }}{{ range .EngineOptions.ArbitraryFlags }}--{{.}} {{ end }}
ExecReload=/bin/kill -s HUP $MAINPID

# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity

# Uncomment TasksMax if your systemd version supports it.
# Only systemd 226 and above support this version.
TasksMax=infinity
TimeoutStartSec=0

# set delegate yes so that systemd does not reset the cgroups of docker containers
Delegate=yes

# kill only the docker process, not all processes in the cgroup
KillMode=process

[Install]
WantedBy=multi-user.target
`
	t, err := template.New("engineConfig").Parse(engineConfigTmpl)
	if err != nil {
		return nil, err
	}

	engineConfigContext := provision.EngineConfigContext{
		DockerPort:    dockerPort,
		AuthOptions:   p.AuthOptions,
		EngineOptions: p.EngineOptions,
	}

	if err := t.Execute(&engineCfg, engineConfigContext); err != nil {
		return nil, err
	}

	return &provision.DockerOptions{
		EngineOptions:     engineCfg.String(),
		EngineOptionsPath: "/lib/systemd/system/docker.service",
	}, nil
}

func (p *BuildrootProvisioner) Package(name string, action pkgaction.PackageAction) error {
	return nil
}

func (p *BuildrootProvisioner) Provision(swarmOptions swarm.Options, authOptions auth.Options, engineOptions engine.Options) error {
	p.SwarmOptions = swarmOptions
	p.AuthOptions = authOptions
	p.EngineOptions = engineOptions

	glog.Infof("provisioning %q", p.Driver.GetMachineName())
	if err := p.SetHostname(p.Driver.GetMachineName()); err != nil {
		return err
	}

	p.AuthOptions = setRemoteAuthOptions(p)
	glog.V(2).Infof("set auth options %+v", p.AuthOptions)

	glog.V(2).Infof("setting up certificates")
	configureAuth := func() error {
		if err := configureAuth(p); err != nil {
			return &util.RetriableError{Err: errors.Wrap(err, "configure auth")}
		}
		return nil
	}
	err := util.RetryAfter(5, configureAuth, time.Second*10)
	if err != nil {
		glog.V(2).Infof("Error configuring auth during provisioning %v", err)
		return err
	}

	glog.V(2).Infof("setting minikube options for container-runtime")
	if err := setMinikubeOptions(p); err != nil {
		glog.V(2).Infof("Error setting container-runtime options during provisioning %v", err)
		return err
	}

	return nil
}

func setRemoteAuthOptions(p provision.Provisioner) auth.Options {
	dockerDir := p.GetDockerOptionsDir()
	authOptions := p.GetAuthOptions()

	// due to windows clients, we cannot use filepath.Join as the paths
	// will be mucked on the linux hosts
	authOptions.CaCertRemotePath = path.Join(dockerDir, "ca.pem")
	authOptions.ServerCertRemotePath = path.Join(dockerDir, "server.pem")
	authOptions.ServerKeyRemotePath = path.Join(dockerDir, "server-key.pem")

	return authOptions
}

func setMinikubeOptions(p *BuildrootProvisioner) error {
	// pass through --insecure-registry
	var (
		crioOptsTmpl = `
CRIO_MINIKUBE_OPTIONS='{{ range .EngineOptions.InsecureRegistry }}--insecure-registry {{.}} {{ end }}'
`
		crioOptsPath = "/etc/sysconfig/crio.minikube"
	)
	t, err := template.New("crioOpts").Parse(crioOptsTmpl)
	if err != nil {
		return err
	}
	var crioOptsBuf bytes.Buffer
	if err := t.Execute(&crioOptsBuf, p); err != nil {
		return err
	}

	if _, err = p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s && printf %%s \"%s\" | sudo tee %s", path.Dir(crioOptsPath), crioOptsBuf.String(), crioOptsPath)); err != nil {
		return err
	}

	// This is unlikely to cause issues unless the user has explicitly requested CRIO, so just log a warning.
	if err := p.Service("crio", serviceaction.Restart); err != nil {
		glog.Warningf("Unable to restart crio service. Error: %v", err)
	}

	return nil
}

func configureAuth(p *BuildrootProvisioner) error {
	glog.Infof("Configuring auth ...")
	driver := p.GetDriver()
	machineName := driver.GetMachineName()
	authOptions := p.GetAuthOptions()
	org := mcnutils.GetUsername() + "." + machineName
	bits := 2048

	ip, err := driver.GetIP()
	if err != nil {
		return errors.Wrap(err, "error getting ip during provisioning")
	}

	local := rexec.NewLocal()
	certs := map[string]string{
		authOptions.CaCertPath:     "ca.pem",
		authOptions.ClientCertPath: "cert.pem",
		authOptions.ClientKeyPath:  "key.pem",
	}
	for src, dst := range certs {
		glog.Infof("cert %s -> %s", src, dst)
		err := local.Copy(src, filepath.Join(authOptions.StorePath, dst), 0750)
		if err != nil {
			return errors.Wrap(err, "copy")
		}
	}

	// The Host IP is always added to the certificate's SANs list
	hosts := append(authOptions.ServerCertSANs, ip, "localhost")
	glog.V(2).Infof("generating server cert: %s ca-key=%s private-key=%s org=%s san=%s",
		authOptions.ServerCertPath,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		org,
		hosts,
	)

	err = cert.GenerateCert(&cert.Options{
		Hosts:     hosts,
		CertFile:  authOptions.ServerCertPath,
		KeyFile:   authOptions.ServerKeyPath,
		CAFile:    authOptions.CaCertPath,
		CAKeyFile: authOptions.CaPrivateKeyPath,
		Org:       org,
		Bits:      bits,
	})

	if err != nil {
		return fmt.Errorf("error generating server cert: %v", err)
	}

	sc, err := sshutil.NewSSHClient(driver)
	if err != nil {
		return errors.Wrap(err, "ssh")
	}
	ssh := rexec.NewSSH(sc)

	remoteCerts := map[string]string{
		authOptions.CaCertPath:     authOptions.CaCertRemotePath,
		authOptions.ServerCertPath: authOptions.ServerCertRemotePath,
		authOptions.ServerKeyPath:  authOptions.ServerKeyRemotePath,
	}
	for src, dst := range remoteCerts {
		if err := ssh.Copy(src, dst, 0640); err != nil {
			return errors.Wrap(err, "copy")
		}
	}

	config, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "load")
	}

	dockerCfg, err := p.GenerateDockerOptions(engine.DefaultPort)
	if err != nil {
		return errors.Wrap(err, "generating docker options")
	}

	console.OutLn("DOCKER!!!!")
	glog.Info("Setting Docker configuration on the remote daemon...")
	if _, err = p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s && printf %%s \"%s\" | sudo tee %s", path.Dir(dockerCfg.EngineOptionsPath), dockerCfg.EngineOptions, dockerCfg.EngineOptionsPath)); err != nil {
		return err
	}

	if config.MachineConfig.ContainerRuntime == "" {
		if err := p.Service("docker", serviceaction.Enable); err != nil {
			return err
		}

		if err := p.Service("docker", serviceaction.Restart); err != nil {
			return err
		}
	}
	return nil
}
