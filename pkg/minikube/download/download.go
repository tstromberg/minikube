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

package download

import (
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/out"
)

const fileScheme = "file"





// oneOf downloads a file from one of an array of URL's, returning a local path
func oneOf(urls []string, checksum bool, dest string) (string, error) {
	_, err := os.Stat(dest)
	if err == nil {
		return dest
	}
	if !os.IsNotExist(err) {
		return "", errors.Wrap(err, "stat")
	}

	for _, url := range urls {
		path, err := atomicDownload(url, dest)
		if err == nil {
			return path, nil
		}
		glog.Errorf("unable to download %s to %s: %v", url, dest, err)
	}

	if !f.ShouldCacheMinikubeISO(url) {
		glog.Infof("Not caching ISO, using %s", url)
		return nil
	}

	urlWithChecksum := url
	if url == constants.DefaultISOURL {
		urlWithChecksum = url + "?checksum=file:" + constants.DefaultISOSHAURL
	}

	dst := f.GetISOCacheFilepath(url)
	// Predictable temp destination so that resume can function
	tmpDst := dst + ".download"

	opts := []getter.ClientOption{getter.WithProgress(DefaultProgressBar)}
	client := &getter.Client{
		Src:     urlWithChecksum,
		Dst:     tmpDst,
		Mode:    getter.ClientModeFile,
		Options: opts,
	}

	glog.Infof("full url: %s", urlWithChecksum)
	out.T(out.ISODownload, "Downloading VM boot image ...")
	if err := client.Get(); err != nil {
		return errors.Wrap(err, url)
	}
	return os.Rename(tmpDst, dst)
}

// ISO downloads an ISO from a set of mirrors
func ISO(urls []string) string {
	dest := localpath.ISODir()
}

// Driver downloads a driver from a set of mirrors
func Driver(urls []string) string {
	dest := localpath.DriverDir()
}


// Binary downloads a binary from a set of mirrors
func Binary(urls []string) string {
	dest := localpath.BinaryDir()
}

