---
title: "minikube start"
linkTitle: "Get Started!"
weight: 1
aliases:
  - /docs/start
---

minikube is local Kubernetes, focusing on making it easy to learn and develop for Kubernetes. 

All you need is Docker (or similarly compatible) container or a Virtual Machine environment, and Kubernetes is a single command away: `minikube start`


## What you’ll need

* 2GB of free memory
* 20GB of free disk space
* Internet connection
* Container or virtual machine manager, such as: [Docker]({{<ref "/docs/drivers/docker">}}), [Hyperkit]({{<ref "/docs/drivers/hyperkit">}}), [Hyper-V]({{<ref "/docs/drivers/hyperv">}}), [KVM]({{<ref "/docs/drivers/kvm2">}}), [Parallels]({{<ref "/docs/drivers/parallels">}}), [Podman]({{<ref "/docs/drivers/podman">}}), [VirtualBox]({{<ref "/docs/drivers/virtualbox">}}), or [VMWare]({{<ref "/docs/drivers/vmware">}})

<h2 class="step"><span class="fa-stack fa-1x"><i class="fa fa-circle fa-stack-2x"></i><strong class="fa-stack-1x text-primary">1</strong></span>Installation</h2>

{{% tabs %}}
{{% tab "Linux" %}}

For Linux users, we provide 3 easy download options:

### Binary download

```shell
 curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
 sudo install minikube-linux-amd64 /usr/local/bin/minikube
```

### Debian package

```shell
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube_{{< latest >}}-0_amd64.deb
sudo dpkg -i minikube_{{< latest >}}-0_amd64.deb
```

### RPM package

```shell
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-{{< latest >}}-0.x86_64.rpm
sudo rpm -ivh minikube-{{< latest >}}-0.x86_64.rpm
```

{{% /tab %}}
{{% tab "macOS" %}}

If the [Brew Package Manager](https://brew.sh/) installed:

```shell
brew install minikube
```

Otherwise, download minikube directly:

```shell
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-darwin-amd64
sudo install minikube-darwin-amd64 /usr/local/bin/minikube
```

{{% /tab %}}
{{% tab "Windows" %}}

If the [Chocolatey Package Manager](https://chocolatey.org/) is installed, use it to install minikube:

```shell
choco install minikube
```

Otherwise, download and run the [Windows installer](https://storage.googleapis.com/minikube/releases/latest/minikube-installer.exe)

{{% /tab %}}
{{% /tabs %}}

<h2 class="step"><span class="fa-stack fa-1x"><i class="fa fa-circle fa-stack-2x"></i><strong class="fa-stack-1x text-primary">2</strong></span>Start your cluster</h2>

From a terminal with administrator access (but not logged in as root), run:

```shell
minikube start
```

If minikube fails to start, see the [drivers page]({{<ref "/docs/drivers">}}) for help setting up a compatible container or virtual-machine manager.

<h2 class="step"><span class="fa-stack fa-1x"><i class="fa fa-circle fa-stack-2x"></i><strong class="fa-stack-1x text-primary">3</strong></span>Interact with your cluster</h2>

If you already have kubectl installed, you can now use it to access your shiny new cluster:

```shell
kubectl get po -A
```

Alternatively, minikube can download the appropriate version of kubectl, if you don't mind the double-dashes in the command-line:

```shell
minikube kubectl -- get po -A
```

minikube bundles the Kubernetes Dashboard, allowing you to get easily acclimated to your new environment:

```shell
minikube dashboard
```

<h2 class="step"><span class="fa-stack fa-1x"><i class="fa fa-circle fa-stack-2x"></i><strong class="fa-stack-1x text-primary">4</strong></span>Deploy applications</h2>

Create a sample deployment and expose it on port 8080:

```shell
kubectl create deployment hello-minikube --image=k8s.gcr.io/echoserver:1.4
kubectl expose deployment hello-minikube --type=NodePort --port=8080
```

It may take a moment, but your deployment will soon show up when you run:

```shell
kubectl get services hello-minikube
```

The easiest way to access this service is to let minikube launch a web browser for you:

```shell
minikube service hello-minikube
```

Alternatively, use kubectl to forward the port:

```shell
kubectl port-forward service/hello-minikube 7080:8080
```

Tada! Your application is now available at [http://localhost:7080/](http://localhost:7080/)

### LoadBalancer deployments

To access a LoadBalancer deployment, use the "minikube tunnel" command. Here is an example deployment:

```shell
kubectl create deployment balanced --image=k8s.gcr.io/echoserver:1.4  
kubectl expose deployment balanced --type=LoadBalancer --port=8000
```

In another window, start the tunnel to create a routable IP for the 'balanced' deployment:

```shell
minikube tunnel
```

To find the routable IP, run this command and examine the `EXTERNAL-IP` column:

`kubectl get services balanced`

Your deployment is now available at &lt;EXTERNAL-IP&gt;:8000

<h2 class="step"><span class="fa-stack fa-1x"><i class="fa fa-circle fa-stack-2x"></i><strong class="fa-stack-1x text-primary">5</strong></span>Manage your cluster</h2>

Pause Kubernetes without impacting deployed applications:

```shell
minikube pause
```

Halt the cluster:

```shell
minikube stop
```

Increase the default memory limit (requires a restart):

```shell
minikube config set memory 16384
```

Browse the catalog of easily installed Kubernetes services:

```shell
minikube addons list
```

Create a second cluster running an older Kubernetes release:

```shell
minikube start -p aged --kubernetes-version=v1.16.1
```

Delete all of the minikube clusters:


```shell
minikube delete --all
```

## Take the next step

* [The minikube handbook]({{<ref "/docs/handbook">}})
* [Community-contributed tutorials]({{<ref "/docs/tutorials">}})
* [minikube command reference]({{<ref "/docs/commands">}})
* [Contributors guide]({{<ref "/docs/contrib">}})
* Take our [fast 5-question survey](https://forms.gle/Gg3hG5ZySw8c1C24A) to share your thoughts 🙏
