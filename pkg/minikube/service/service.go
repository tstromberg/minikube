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

package service

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/machine/libmachine"
	"github.com/golang/glog"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	"text/template"

	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/labels"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/util"
)

type K8sClient interface {
	GetCoreClient() (corev1.CoreV1Interface, error)
	GetClientset() (*kubernetes.Clientset, error)
}

type K8sClientGetter struct{}

var K8s K8sClient

func init() {
	K8s = &K8sClientGetter{}
}

func (k *K8sClientGetter) GetCoreClient() (corev1.CoreV1Interface, error) {
	client, err := k.GetClientset()
	if err != nil {
		return nil, errors.Wrap(err, "getting clientset")
	}
	return client.Core(), nil
}

func (*K8sClientGetter) GetClientset() (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	profile := viper.GetString(config.MachineProfile)
	configOverrides := &clientcmd.ConfigOverrides{
		Context: clientcmdapi.Context{
			Cluster:  profile,
			AuthInfo: profile,
		},
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("Error creating kubeConfig: %v", err)
	}
	clientConfig.Timeout = 1 * time.Second
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating new client from kubeConfig.ClientConfig()")
	}

	return client, nil
}

type URL struct {
	Namespace string
	Name      string
	URLs      []string
}

type URLs []URL

// Returns all the node port URLs for every service in a particular namespace
// Accepts a template for formatting
func GetServiceURLs(api libmachine.API, namespace string, t *template.Template) (URLs, error) {
	host, err := cluster.CheckIfHostExistsAndLoad(api, config.GetMachineName())
	if err != nil {
		return nil, err
	}

	ip, err := host.Driver.GetIP()
	if err != nil {
		return nil, err
	}

	client, err := K8s.GetCoreClient()
	if err != nil {
		return nil, err
	}

	serviceInterface := client.Services(namespace)

	svcs, err := serviceInterface.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var serviceURLs []URL
	for _, svc := range svcs.Items {
		urls, err := printURLsForService(client, ip, svc.Name, svc.Namespace, t)
		if err != nil {
			return nil, err
		}
		serviceURLs = append(serviceURLs, URL{Namespace: svc.Namespace, Name: svc.Name, URLs: urls})
	}

	return serviceURLs, nil
}

// Returns all the node ports for a service in a namespace
// with optional formatting
func GetServiceURLsForService(api libmachine.API, namespace, service string, t *template.Template) ([]string, error) {
	host, err := cluster.CheckIfHostExistsAndLoad(api, config.GetMachineName())
	if err != nil {
		return nil, errors.Wrap(err, "Error checking if api exist and loading it")
	}

	ip, err := host.Driver.GetIP()
	if err != nil {
		return nil, errors.Wrap(err, "Error getting ip from host")
	}

	client, err := K8s.GetCoreClient()
	if err != nil {
		return nil, err
	}

	return printURLsForService(client, ip, service, namespace, t)
}

func printURLsForService(c corev1.CoreV1Interface, ip, service, namespace string, t *template.Template) ([]string, error) {
	if t == nil {
		return nil, errors.New("Error, attempted to generate service url with nil --format template")
	}

	s := c.Services(namespace)
	svc, err := s.Get(service, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "service '%s' could not be found running", service)
	}
	var nodePorts []int32
	if len(svc.Spec.Ports) > 0 {
		for _, port := range svc.Spec.Ports {
			if port.NodePort > 0 {
				nodePorts = append(nodePorts, port.NodePort)
			}
		}
	}
	urls := []string{}
	for _, port := range nodePorts {
		var doc bytes.Buffer
		err = t.Execute(&doc, struct {
			IP   string
			Port int32
		}{
			ip,
			port,
		})
		if err != nil {
			return nil, err
		}

		urls = append(urls, doc.String())
	}
	return urls, nil
}

// CheckService checks if a service is listening on a port.
func CheckService(namespace string, service string) error {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return errors.Wrap(err, "Error getting kubernetes client")
	}

	svc, err := client.Services(namespace).Get(service, metav1.GetOptions{})
	if err != nil {
		return &util.RetriableError{
			Err: errors.Wrapf(err, "Error getting service %s", service),
		}
	}
	if len(svc.Spec.Ports) == 0 {
		return fmt.Errorf("%s:%s has no ports", namespace, service)
	}
	glog.Infof("Found service: %+v", svc)
	return nil
}

func OptionallyHTTPSFormattedURLString(bareURLString string, https bool) (string, bool) {
	httpsFormattedString := bareURLString
	isHTTPSchemedURL := false

	if u, parseErr := url.Parse(bareURLString); parseErr == nil {
		isHTTPSchemedURL = u.Scheme == "http"
	}

	if isHTTPSchemedURL && https {
		httpsFormattedString = strings.Replace(bareURLString, "http", "https", 1)
	}

	return httpsFormattedString, isHTTPSchemedURL
}

func WaitAndMaybeOpenService(api libmachine.API, namespace string, service string, urlTemplate *template.Template, urlMode bool, https bool,
	wait int, interval int) error {
	if err := util.RetryAfter(wait, func() error { return CheckService(namespace, service) }, time.Duration(interval)*time.Second); err != nil {
		return errors.Wrapf(err, "Could not find finalized endpoint being pointed to by %s", service)
	}

	urls, err := GetServiceURLsForService(api, namespace, service, urlTemplate)
	if err != nil {
		return errors.Wrap(err, "Check that minikube is running and that you have specified the correct namespace")
	}
	for _, bareURLString := range urls {
		urlString, isHTTPSchemedURL := OptionallyHTTPSFormattedURLString(bareURLString, https)

		if urlMode || !isHTTPSchemedURL {
			fmt.Fprintln(os.Stdout, urlString)
		} else {
			fmt.Fprintln(os.Stderr, "Opening kubernetes service "+namespace+"/"+service+" in default browser...")
			browser.OpenURL(urlString)
		}
	}
	return nil
}

func GetServiceListByLabel(namespace string, key string, value string) (*v1.ServiceList, error) {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return &v1.ServiceList{}, &util.RetriableError{Err: err}
	}
	services := client.Services(namespace)
	if err != nil {
		return &v1.ServiceList{}, &util.RetriableError{Err: err}
	}
	return getServiceListFromServicesByLabel(services, key, value)
}

func getServiceListFromServicesByLabel(services corev1.ServiceInterface, key string, value string) (*v1.ServiceList, error) {
	selector := labels.SelectorFromSet(labels.Set(map[string]string{key: value}))
	serviceList, err := services.List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return &v1.ServiceList{}, &util.RetriableError{Err: err}
	}

	return serviceList, nil
}

// CreateSecret creates or modifies secrets
func CreateSecret(namespace, name string, dataValues map[string]string, labels map[string]string) error {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return &util.RetriableError{Err: err}
	}
	secrets := client.Secrets(namespace)
	if err != nil {
		return &util.RetriableError{Err: err}
	}

	secret, _ := secrets.Get(name, metav1.GetOptions{})

	// Delete existing secret
	if len(secret.Name) > 0 {
		err = DeleteSecret(namespace, name)
		if err != nil {
			return &util.RetriableError{Err: err}
		}
	}

	// convert strings to data secrets
	data := map[string][]byte{}
	for key, value := range dataValues {
		data[key] = []byte(value)
	}

	// Create Secret
	secretObj := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Data: data,
		Type: v1.SecretTypeOpaque,
	}

	_, err = secrets.Create(secretObj)
	if err != nil {
		fmt.Println("err: ", err)
		return &util.RetriableError{Err: err}
	}

	return nil
}

// DeleteSecret deletes a secret from a namespace
func DeleteSecret(namespace, name string) error {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return &util.RetriableError{Err: err}
	}

	secrets := client.Secrets(namespace)
	if err != nil {
		return &util.RetriableError{Err: err}
	}

	err = secrets.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return &util.RetriableError{Err: err}
	}

	return nil
}
