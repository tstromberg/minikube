# Toward rotating debug logs

* Proposed: 2020-09-24
* Authors: Thomas Stromberg (@tstromberg)

## Reviewer Priorities

Please review this proposal with the following priorities:

*   Does this fit with minikube's [principles](https://minikube.sigs.k8s.io/docs/concepts/principles/)?
*   Are there other approaches to consider?
*   Could the implementation be made simpler?
*   Are there usability, reliability, or technical debt concerns?

Please leave the above text in your proposal as instructions to the reader.

## Summary

With a `minikube status` command executed every 5 seconds, minkube will consume 51843 inodes within a 24 hour period. This MEP proposes to change our logging structure so that repeated runs do not consume all available inodes.

We've seen multiple user reports over the years which have complained about minikube logs taking over $TMPDIR. This stems from our usage of [google/glog](https://github.com/golang/glog) library, which was designed assuming that a scheduled cleanup task cleans up after itself. Every time a `minikube` command is run, it creates 3 files, and updates 3 symbolic links:

```
minikube.tstromberg-macbookpro.tstromberg.log.WARNING.20200924-065231.11894
minikube.tstromberg-macbookpro.tstromberg.log.ERROR.20200924-065231.11894
minikube.tstromberg-macbookpro.tstromberg.log.INFO.20200924-065231.11894
minikube.ERROR -> minikube.tstromberg-macbookpro.tstromberg.log.ERROR.20200924-065231.11894
minikube.INFO -> minikube.tstromberg-macbookpro.tstromberg.log.INFO.20200924-065231.11894
minikube.WARNING -> minikube.tstromberg-macbookpro.tstromberg.log.WARNING.20200924-065231.11894
```


With the advent of UI's which monitor the state of a Kubernetes cluster every couple of seconds, and the lack of a program cleaning up $TMPDIR, this disk layout can quickly cause an inode allocation issue. On many filesystems, inodes cannot be grown without recreating the filesystem, so they must be carefully managed.

## Goals

*   Given an infinite number of minikube commands, consume no more than 1000 inodes
*   Make it possible to find individual logs for bug reports

## Non-Goals

*   Introduce any new commands or alter the behavior of existing commands
*   Store additional logs
*   Design a system that requires a background cleanup task

## Design Details

Switch minikube's debug log implementaio from [google/glog] to a fork that allows an explicit  destination, such as [kubernetes/klog](https://github.com/kubernetes/klog). 



For example, we could use a destination path of:

`$HOME/.minikube/logs/<command>_%d.log`




_(2+ paragraphs) A short overview of your implementation idea, containing only as much detail as required to convey your idea._

_If you have multiple ideas, list them concisely._

_Include a testing plan to ensure that your enhancement is not broken by future changes._

## Alternatives Considered

### Ascending Index Rotation

Save to `$HOME/.minikube/logs/%s_%d.log`, where `%s` is the command being run, such as `start`, and `%d` is the index, for instance 0, for the most recent log. Subsequent commands would rename the previous log to `index+1`. 

This approach will however fail on Windows if the previous command has not yet completed, as it is not possible to rename open files.


### Ascending Index Reuse

Save to `$HOME/.minikube/logs/%s_%d.log`, where `%s` is the command being run, such as `start`, and `%d` is the index. 

 for instance 0, for the most recent log. Subsequent commands would rename the previous log to `index+1`. 

This approach will however fail on Windows if the previous command has not yet completed, as it is not possible to rename open files.


### Background cleanup

One posssibility would be to 

Always save to `minikube_start.0.log` and 

_Alternative ideas that you are leaning against._
