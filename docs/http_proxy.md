## Using Minikube with an HTTP Proxy

minikube requires access to the internet via HTTP, HTTPS, and DNS protocols. If a HTTP proxy is required to access the internet, you may need to pass the proxy connection information to both minikube and Docker using environment variables:

* `HTTP_PROXY` - The URL to your HTTP proxy
* `HTTPS_PROXY` - The URL to your HTTPS proxy
* `NO_PROXY` - A comma-separated list of hosts which should not go through the proxy.

The NO_PROXY variable here is important: Without setting it, minikube may not be able to access resources within the VM. minikube uses two IP ranges, which should not go through the proxy:

* **192.168.99.1/24**: Used by the minikube VM. Configurable for some hypervisors via `--host-only-cidr`
* **10.96.0.0/12**: Used by service cluster IP's. Configurable via  `--service-cluster-ip-range`

## Example Usage

### macOS and Linux

```
export HTTP_PROXY=http://<proxy hostname:port>
export HTTPS_PROXY=https://<proxy hostname:port>
export NO_PROXY=localhost,127.0.0.1,10.96.0.0/12,192.168.99.1/24

minikube start --docker-env=HTTP_PROXY=$HTTP_PROXY --docker-env HTTPS_PROXY=$HTTPS_PROXY \
  --docker-env NO_PROXY=$NO_PROXY
```

To make the exported variables permanent, consider adding the declarations to ~/.bashrc or wherever your user-set environment variables are stored.

### Windows

```
set HTTP_PROXY=http://<proxy hostname:port>
set HTTPS_PROXY=https://<proxy hostname:port>
set NO_PROXY=localhost,127.0.0.1,10.96.0.0/12,192.168.99.1/24

minikube start --docker-env=HTTP_PROXY=$HTTP_PROXY --docker-env HTTPS_PROXY=$HTTPS_PROXY \
  --docker-env NO_PROXY=$NO_PROXY
```

To set these environment variables permanently, consider adding these to your [system settings](https://support.microsoft.com/en-au/help/310519/how-to-manage-environment-variables-in-windows-xp) or using [setx](https://stackoverflow.com/questions/5898131/set-a-persistent-environment-variable-from-cmd-exe)

## Debugging

### unable to cache ISO... connection refused

```
Unable to start VM: unable to cache ISO: https://storage.googleapis.com/minikube/iso/minikube.iso:
failed to download: failed to download to temp file: download failed: 5 error(s) occurred:

* Temporary download error: Get https://storage.googleapis.com/minikube/iso/minikube.iso:
proxyconnect tcp: dial tcp <host>:<port>: connect: connection refused
```

This error indicates that the host:port combination defined by HTTPS_PROXY or HTTP_PROXY is incorrect, or that the proxy is unavailable.

## Unable to pull images..Client.Timeout exceeded while awaiting headers

```
Unable to pull images, which may be OK: 

failed to pull image "k8s.gcr.io/kube-apiserver:v1.13.3": output: Error response from daemon:
Get https://k8s.gcr.io/v2/: net/http: request canceled while waiting for connection
(Client.Timeout exceeded while awaiting headers)
```

This error indicates that the container runtime running within the VM does not have access to the internet. Verify that you are passing the appropriate value to `--docker-env HTTPS_PROXY`.

## x509: certificate signed by unknown authority

```
[ERROR ImagePull]: failed to pull image k8s.gcr.io/kube-apiserver:v1.13.3:
output: Error response from daemon:
Get https://k8s.gcr.io/v2/: x509: certificate signed by unknown authority
```

This is because minikube VM is stuck behind a proxy that rewrites HTTPS responses to contain its own TLS certificate. The [solution](https://github.com/kubernetes/minikube/issues/3613#issuecomment-461034222) is to install the proxy certificate into a location that is copied to the VM at startup, so that it can be validated.

Ask your IT department for the appropriate PEM file, and add it to:

`~/.minikube/files/etc/ssl/certs`

Then run `minikube delete` and `minikube start`.


## Additional Information

- [Configure Docker to use a proxy server](https://docs.docker.com/network/proxy/)
