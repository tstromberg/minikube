package cruntime

import (
	"bytes"
	"fmt"
	"html/template"
	"path"

	"github.com/golang/glog"
)

// CRIO contains CRIO runtime state
type CRIO struct {
	Socket string
	Runner CommandRunner
}

// Name is a human readable name for CRIO
func (r *CRIO) Name() string {
	return "CRIO"
}

// SocketPath returns the path to the socket file for CRIO
func (r *CRIO) SocketPath() string {
	if r.Socket != "" {
		return r.Socket
	}
	return "/var/run/crio/crio.sock"
}

// Available returns an error if it is not possible to use this runtime on a host
func (r *CRIO) Available() error {
	return r.Runner.Run("command -v crio")
}

// Active returns if CRIO is active on the host
func (r *CRIO) Active() bool {
	err := r.Runner.Run("systemctl is-active --quiet service crio")
	return err == nil
}

// createConfigFile runs the commands necessary to create crictl.yaml
func (r *CRIO) createConfigFile() error {
	var (
		crictlYamlTmpl = `runtime-endpoint: {{.RuntimeEndpoint}}
image-endpoint: {{.ImageEndpoint}}
`
		crictlYamlPath = "/etc/crictl.yaml"
	)
	t, err := template.New("crictlYaml").Parse(crictlYamlTmpl)
	if err != nil {
		return err
	}
	opts := struct {
		RuntimeEndpoint string
		ImageEndpoint   string
	}{
		RuntimeEndpoint: r.SocketPath(),
		ImageEndpoint:   r.SocketPath(),
	}
	var crictlYamlBuf bytes.Buffer
	if err := t.Execute(&crictlYamlBuf, opts); err != nil {
		return err
	}
	return r.Runner.Run(fmt.Sprintf("sudo mkdir -p %s && printf %%s \"%s\" | sudo tee %s",
		path.Dir(crictlYamlPath), crictlYamlBuf.String(), crictlYamlPath))
}

// Enable idempotently enables CRIO on a host
func (r *CRIO) Enable() error {
	if err := disableOthers(r, r.Runner); err != nil {
		glog.Warningf("disableOthers: %v", err)
	}
	if err := r.createConfigFile(); err != nil {
		return err
	}
	if err := enableIPForwarding(r.Runner); err != nil {
		return err
	}
	return r.Runner.Run("sudo systemctl restart crio")
}

// Disable idempotently disables CRIO on a host
func (r *CRIO) Disable() error {
	return r.Runner.Run("sudo systemctl stop crio")
}

// LoadImage loads an image into this runtime
func (r *CRIO) LoadImage(path string) error {
	return r.Runner.Run(fmt.Sprintf("sudo podman load -i %s", path))
}

// KubeletOptions returns kubelet options for a runtime.
func (r *CRIO) KubeletOptions() map[string]string {
	return map[string]string{
		"container-runtime":          "remote",
		"container-runtime-endpoint": r.SocketPath(),
		"image-service-endpoint":     r.SocketPath(),
		"runtime-request-timeout":    "15m",
	}
}

// ListContainers returns a list of managed by this container runtime
func (r *CRIO) ListContainers(filter string) ([]string, error) {
	return listCRIContainers(r.Runner, filter)
}

// KillContainers removes containers based on ID
func (r *CRIO) KillContainers(ids []string) error {
	return killCRIContainers(r.Runner, ids)
}

// StopContainers stops containers based on ID
func (r *CRIO) StopContainers(ids []string) error {
	return stopCRIContainers(r.Runner, ids)
}
