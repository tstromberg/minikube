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

const (
	// SHASuffix is the suffix of a SHA-256 checksum file
	SHASuffix = ".sha256"
)


// DefaultISOURLs returns the default URL's used for an ISO download, semicolon delimited
func DefaultISOURLs() string {
	urls := []string{
		fmt.Sprintf("https://github.com/kubernetes/minikube/releases/download/%s/minikube-%s.iso", minikubeVersion.GetISOVersion(), minikubeVersion.GetISOVersion()),
		fmt.Sprintf("https://storage.googleapis.com/%s/minikube-%s.iso", minikubeVersion.GetISOPath(), minikubeVersion.GetISOVersion()),
	}
	return strings.Join(";", urls)
}

// oneOf downloads a file from one of an array of URL's, returning a local path
func oneOf(urls []string, checksum bool, dest string) (string, error) {
}

func download(src string, dest string) error {
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
}

// ISO downloads an ISO from a set of mirrors
func ISO(urls []string) string {
	dest := localpath.ISODir()
}

