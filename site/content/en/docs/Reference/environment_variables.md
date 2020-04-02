---
title: "Environment Variables"
linkTitle: "Environment Variables"
weight: 6
date: 2019-08-01
---

## Config option variables

minikube supports passing environment variables instead of flags for every value listed in `minikube config`.  This is done by passing an environment variable with the prefix `MINIKUBE_`.

For example the `minikube start --iso-url="$ISO_URL"` flag can also be set by setting the `MINIKUBE_ISO_URL="$ISO_URL"` environment variable.

## Other variables

Some features can only be accessed by minikube specific environment variables, here is a list of these features:

* **MINIKUBE_HOME** - (string) sets the path for the .minikube directory that minikube uses for state/configuration. *Please note: this is used only by minikube and does not affect anything related to Kubernetes tools such as kubectl.*

* **MINIKUBE_IN_STYLE** - (bool) manually sets whether or not emoji and colors should appear in minikube. Set to false or 0 to disable this feature, true or 1 to force it to be turned on.

* **MINIKUBE_WANTUPDATENOTIFICATION** - (bool) sets whether the user wants an update notification for new minikube versions

* **MINIKUBE_REMINDERWAITPERIODINHOURS** - (int) sets the number of hours to check for an update notification

* **CHANGE_MINIKUBE_NONE_USER** - (bool) automatically change ownership of ~/.minikube to the value of $SUDO_USER

* **MINIKUBE_ENABLE_PROFILING** - (int, `1` enables it) enables trace profiling to be generated for minikube


## Example: Disabling emoji

```shell
export MINIKUBE_IN_STYLE=false
minikube start
```

## Making values persistent

To make the exported variables persistent across reboots:

* Linux and macOS: Add these declarations to `~/.bashrc` or wherever your shells environment variables are stored.
* Windows: Add these declarations via [system settings](https://support.microsoft.com/en-au/help/310519/how-to-manage-environment-variables-in-windows-xp) or using [setx](https://stackoverflow.com/questions/5898131/set-a-persistent-environment-variable-from-cmd-exe)

