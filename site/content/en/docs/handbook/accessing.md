---
title: "Accessing apps"
weight: 3
description: >
  How to access applications running within minikube
aliases:
 - /docs/tasks/loadbalancer
 - /Handbook/loadbalancer/
 - /docs/tasks/nodeport
---

There are two major categories of services in Kubernetes:

* NodePort
* LoadBalancer

minikube supports either. Read on!

## NodePort access

A NodePort service is the most basic way to get external traffic directly to your service. NodePort, as the name implies, opens a specific port, and any traffic that is sent to this port is forwarded to the service.

### Getting the NodePort using the service command

We also have a shortcut for fetching the minikube IP and a service's `NodePort`:

`minikube service --url $SERVICE`

## Getting the NodePort using kubectl

The minikube VM is exposed to the host system via a host-only IP address, that can be obtained with the `minikube ip` command. Any services of type `NodePort` can be accessed over that IP address, on the NodePort.

To determine the NodePort for your service, you can use a `kubectl` command like this (note that `nodePort` begins with lowercase `n` in JSON output):

`kubectl get service $SERVICE --output='jsonpath="{.spec.ports[0].nodePort}"'`

### Increasing the NodePort range

By default, minikube only exposes ports 30000-32767. If this does not work for you, you can adjust the range by using:

`minikube start --extra-config=apiserver.service-node-port-range=1-65535`

This flag also accepts a comma separated list of ports and port ranges.

----
## LoadBalancer access

A LoadBalancer service is the standard way to expose a service to the internet. With this method, each service gets its own IP address.


## Using `minikube tunnel`

Services of type `LoadBalancer` can be exposed via the `minikube tunnel` command. It must be run in a separate terminal window to keep the `LoadBalancer` running.  Ctrl-C in the terminal can be used to terminate the process at which time the network routes will be cleaned up.

## Example

#### Run tunnel in a separate terminal
it will ask for password.

```
minikube tunnel
```

`minikube tunnel` runs as a process, creating a network route on the host to the service CIDR of the cluster using the cluster's IP address as a gateway.  The tunnel command exposes the external IP directly to any program running on the host operating system.


<details>
<summary>
tunnel output example
</summary>
<pre>
Password: 
Status:	
	machine: minikube
	pid: 39087
	route: 10.96.0.0/12 -> 192.168.64.194
	minikube: Running
	services: [hello-minikube]
    errors: 
		minikube: no errors
		router: no errors
		loadbalancer emulator: no errors
...
...
...
</pre>
</details>


#### Create a kubernetes deployment 
```
kubectl create deployment hello-minikube1 --image=k8s.gcr.io/echoserver:1.4
```
#### Create a kubernetes service type LoadBalancer
```
kubectl expose deployment hello-minikube1 --type=LoadBalancer --port=8080
```

### Check external IP 
```
kubectl get svc
```
<pre>
$ kc get svc
NAME              TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)          AGE
hello-minikube1   LoadBalancer   10.96.184.178   10.96.184.178   8080:30791/TCP   40s
</pre>


note that without minikube tunnel, kubernetes would be showing external IP as "pending".

### Try in your browser
open in your browser (make sure there is no proxy set)
```
http://REPLACE_WITH_EXTERNAL_IP:8080
```


Each service will get its own external ip.

----
### DNS resolution (experimental)

If you are on macOS, the tunnel command also allows DNS resolution for Kubernetes services from the host.

### Cleaning up orphaned routes

If the `minikube tunnel` shuts down in an abrupt manner, it may leave orphaned network routes on your system. If this happens, the ~/.minikube/tunnels.json file will contain an entry for that tunnel. To remove orphaned routes, run:

````shell
minikube tunnel --cleanup
````

### Avoiding password prompts

Adding a route requires root privileges for the user, and thus there are differences in how to run `minikube tunnel` depending on the OS. If you want to avoid entering the root password, consider setting NOPASSWD for "ip" and "route" commands:

<https://superuser.com/questions/1328452/sudoers-nopasswd-for-single-executable-but-allowing-others>
