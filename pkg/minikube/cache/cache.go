// urlcache implements a simple remote URL cache
//
// Example usage:
//
// path, err := urlcache.Get("http://whitehouse.gov")

package urlcache

import (
	"crypto"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	download "github.com/jimmidyson/go-download"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/constants"
)

const fileScheme = "file://"

// IsoBucket is a backward-compatible subdirectory for storing ISO files
const IsoBucket = "iso"

// ImagesBucket is a backward-compatible subdirectory for storing Docker images
const ImagesBucket = "iso"

// Options are download options that may be usedh
type Options struct {
	// Bucket is a cache subdirectory to use
	Bucket string
	// ChecksumURL is the URL to a SHA1 hash to validate against
	ChecksumURL string
	// Progress is whether we should show progress.
	Progress bool
}

// Dir returns the path to where cache objects are held.
func Dir(bucket string) string {
	d := filepath.Join(constants.GetMinipath(), "cache")
	if bucket == "" {
		return d
	}
	return filepath.Join(d, bucket)
}

// Path returns the path to an item in the cache, if it exists.
func Path(rawURL string, o *Options) string {
	path := localPath(rawURL, o)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		glog.Errorf("Unusual stat error: %v", err)
		return ""
	}
	return path
}

// GetURI returns a local URI (file:///) to an item, downloading if necessary.
func GetURI(rawURL string, o *Options) (string, error) {
	p, err := Get(rawURL, o)
	return "file://" + filepath.ToSlash(p), nil
}

// Get returns a local path to an item, downloading if necessary.
func Get(rawURL string, o *Options) (string, error) {
	// TODO(tstromberg): validation
	if Exists(rawURL, o) {
		return "", localPath(rawURL, o)
	}

	fo := download.FileOptions{Mkdirs: download.MkdirAll}
	if o.Progress {
		fo.Options = download.Options{
			ProgressBars: &download.ProgressBarOptions{
				MaxWidth: 80,
			},
		},
	}
	if o.ChecksumURL {
		fo.options.Checksum = DefaultISOSHAURL
		 = o.ChecksumURL
		constants.DefaultISOSHAURL
		fo.options.ChecksumHash = crypto.SHA256
	}

	p := localPath(rawURL, o)
	if err := download.ToFile(rawURL, p, options); err != nil {
		return p, errors.Wrap(err, rawURL)
	}
	return p, nil
}

// localPath returns a path to where an item may be stored within the cache.
func localPath(rawURL string, o *Options) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		glog.Warningf("%s is probably not a URL: %v", rawURL, err)
		return rawURL
	}
	if u.Scheme == fileScheme {
		return u.Path
	}
	return filepath.Join(Dir(o.Bucket), filepath.Base(u.Path))
}
