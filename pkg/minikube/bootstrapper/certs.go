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

package bootstrapper

import (
	"bytes"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/rexec"
	"k8s.io/minikube/pkg/util"
	"k8s.io/minikube/pkg/util/kubeconfig"
)

var (
	// installCerts is a list of certificate paths to install
	installCerts = []string{
		"ca.crt",
		"ca.key",
		"apiserver.crt",
		"apiserver.key",
		"proxy-client-ca.crt",
		"proxy-client-ca.key",
		"proxy-client.crt",
		"proxy-client.key",
	}
)

// SetupCerts generates and uploads certificate data into the VM
func SetupCerts(w rexec.Writer, k8s config.KubernetesConfig) error {
	glog.Infof("Setting up certificates for IP: %s\n", k8s.NodeIP)

	if err := generateCerts(k8s); err != nil {
		return errors.Wrap(err, "Error generating certs")
	}
	localPath := constants.GetMinipath()
	for _, name := range installCerts {
		perms := os.FileMode(0644)
		if strings.HasSuffix(name, ".key") {
			perms = os.FileMode(0600)
		}
		if err := w.Copy(filepath.Join(localPath, name), filepath.Join(util.DefaultCertPath, name), perms); err != nil {
			return err
		}
	}
	return installKubeCfg(w, k8s)
}

// installKubeCfg remotely installs a kubecfg, populated with certificate information.
func installKubeCfg(w rexec.Writer, k8s config.KubernetesConfig) error {
	kubeCfgSetup := &kubeconfig.KubeConfigSetup{
		ClusterName:          k8s.NodeName,
		ClusterServerAddress: "https://localhost:8443",
		ClientCertificate:    path.Join(util.DefaultCertPath, "apiserver.crt"),
		ClientKey:            path.Join(util.DefaultCertPath, "apiserver.key"),
		CertificateAuthority: path.Join(util.DefaultCertPath, "ca.crt"),
		KeepContext:          false,
	}

	kubeCfg := api.NewConfig()
	err := kubeconfig.PopulateKubeConfig(kubeCfgSetup, kubeCfg)
	if err != nil {
		return errors.Wrap(err, "populating kubeconfig")
	}
	data, err := runtime.Encode(latest.Codec, kubeCfg)
	if err != nil {
		return errors.Wrap(err, "encoding kubeconfig")
	}
	return w.WriteFile(bytes.NewReader(data), filepath.Join(util.DefaultMinikubeDirectory, "kubeconfig"), int64(len(data)), 0644)
}

// generateCerts generates self-signed certificates on local disk based on a KubernetesCOnfig
func generateCerts(k8s config.KubernetesConfig) error {
	serviceIP, err := util.GetServiceClusterIP(k8s.ServiceCIDR)
	if err != nil {
		return errors.Wrap(err, "getting service cluster ip")
	}

	localPath := constants.GetMinipath()

	caCertPath := filepath.Join(localPath, "ca.crt")
	caKeyPath := filepath.Join(localPath, "ca.key")

	proxyClientCACertPath := filepath.Join(localPath, "proxy-client-ca.crt")
	proxyClientCAKeyPath := filepath.Join(localPath, "proxy-client-ca.key")

	caCertSpecs := []struct {
		certPath string
		keyPath  string
		subject  string
	}{
		{ // client / apiserver CA
			certPath: caCertPath,
			keyPath:  caKeyPath,
			subject:  "minikubeCA",
		},
		{ // proxy-client CA
			certPath: proxyClientCACertPath,
			keyPath:  proxyClientCAKeyPath,
			subject:  "proxyClientCA",
		},
	}

	apiServerIPs := append(
		k8s.APIServerIPs,
		[]net.IP{net.ParseIP(k8s.NodeIP), serviceIP, net.ParseIP("10.0.0.1")}...)
	apiServerNames := append(k8s.APIServerNames, k8s.APIServerName)
	apiServerAlternateNames := append(
		apiServerNames,
		util.GetAlternateDNS(k8s.DNSDomain)...)

	signedCertSpecs := []struct {
		certPath       string
		keyPath        string
		subject        string
		ips            []net.IP
		alternateNames []string
		caCertPath     string
		caKeyPath      string
	}{
		{ // Client cert
			certPath:       filepath.Join(localPath, "client.crt"),
			keyPath:        filepath.Join(localPath, "client.key"),
			subject:        "minikube-user",
			ips:            []net.IP{},
			alternateNames: []string{},
			caCertPath:     caCertPath,
			caKeyPath:      caKeyPath,
		},
		{ // apiserver serving cert
			certPath:       filepath.Join(localPath, "apiserver.crt"),
			keyPath:        filepath.Join(localPath, "apiserver.key"),
			subject:        "minikube",
			ips:            apiServerIPs,
			alternateNames: apiServerAlternateNames,
			caCertPath:     caCertPath,
			caKeyPath:      caKeyPath,
		},
		{ // aggregator proxy-client cert
			certPath:       filepath.Join(localPath, "proxy-client.crt"),
			keyPath:        filepath.Join(localPath, "proxy-client.key"),
			subject:        "aggregator",
			ips:            []net.IP{},
			alternateNames: []string{},
			caCertPath:     proxyClientCACertPath,
			caKeyPath:      proxyClientCAKeyPath,
		},
	}

	for _, caCertSpec := range caCertSpecs {
		if !(util.CanReadFile(caCertSpec.certPath) &&
			util.CanReadFile(caCertSpec.keyPath)) {
			if err := util.GenerateCACert(
				caCertSpec.certPath, caCertSpec.keyPath, caCertSpec.subject,
			); err != nil {
				return errors.Wrap(err, "Error generating CA certificate")
			}
		}
	}

	for _, signedCertSpec := range signedCertSpecs {
		if err := util.GenerateSignedCert(
			signedCertSpec.certPath, signedCertSpec.keyPath, signedCertSpec.subject,
			signedCertSpec.ips, signedCertSpec.alternateNames,
			signedCertSpec.caCertPath, signedCertSpec.caKeyPath,
		); err != nil {
			return errors.Wrap(err, "Error generating signed apiserver serving cert")
		}
	}

	return nil
}
