---
title: "delete"
description: >
  Deletes a local kubernetes cluster
---



## minikube delete

Deletes a local kubernetes cluster

### Synopsis

Deletes a local kubernetes cluster. This command deletes the VM, and removes all
associated files.

```
minikube delete [flags]
```

### Options

```
      --all     Set flag to delete all profiles
  -h, --help    help for delete
      --purge   Set this flag to delete the '.minikube' folder from your user directory.
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
  -b, --bootstrapper string              The name of the cluster bootstrapper that will set up the kubernetes cluster. (default "kubeadm")
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
  -p, --profile string                   The name of the minikube VM being used. This can be set to allow having multiple instances of minikube independently. (default "minikube")
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

