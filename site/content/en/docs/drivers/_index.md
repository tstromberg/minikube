---
title: "Drivers"
linkTitle: "Drivers"
weight: 8
no_list: true
description: >
  Configuring various minikube drivers
aliases:
  - /docs/reference/drivers
---
minikube can be deployed as a VM, a container, or bare-metal.

To do so, we use the [Docker Machine](https://github.com/docker/machine) library to provide a consistent way to interact with different environments. Here is what's supported:

## Linux

* [Docker]({{<ref "docker.md">}}) - container-based (preferred)
* [KVM2]({{<ref "kvm2.md">}}) - VM-based (preferred)
* [VirtualBox]({{<ref "virtualbox.md">}}) - VM
* [None]({{<ref "none.md">}}) -  bare-metal
* [Podman]({{<ref "podman.md">}}) - container (experimental)

## macOS

* [Hyperkit]({{<ref "hyperkit.md">}}) - VM (preferred)
* [Docker]({{<ref "docker.md">}}) - VM + Container
* [VirtualBox]({{<ref "virtualbox.md">}}) - FVM
* [Parallels]({{<ref "parallels.md">}}) - VM
* [VMware]({{<ref "vmware.md">}}) - VM

## Windows

* [Hyper-V]({{<ref "hyperv.md">}}) - VM (preferred)
* [Docker]({{<ref "docker.md">}}) - VM + Container (preferred)
* [VirtualBox]({{<ref "virtualbox.md">}}) - VM
