# Steps to Release Minikube

## Build a new ISO

You only need to build the minikube ISO when the there are changes in the `deploy/iso` folder. 
Note: you can build the ISO using the `hack/jenkins/build_iso.sh` script locally.

 * navigate to the minikube ISO jenkins job
 * Ensure that you are logged in (top right)
 * For `ISO_VERSION`, type in the intended release version (same as the minikube binary's version)
 * For `ISO_BUCKET`, type in `minikube/iso`
 * Click *Build*

The build will take roughly 50 minutes.

## Update Makefile

Edit the minikube `Makefile`, updating the version number values at the top:

* `VERSION_MINOR` (and `VERSION_MAJOR`, `VERSION_BUILD` as necessary)
* `ISO_VERSION` (only update this if there is a new ISO release)

## Run Local Integration Test

With the updated Makefile, run the integration tests and ensure that all tests pass:

```shell
env TEST_ARGS="-minikube-start-args=--vm-driver=kvm2" make integration
```

## Ad-Hoc testing of other platforms

If there are supported platforms which do not have functioning Jenkins workers (Windows), you may use the following to build a sanity check:

```shell
BUILD_IN_DOCKER=y make cross checksum
```

## Send out Makefile PR

This will update users of HEAD to the new ISO.

Please pay attention to test failures, as this is our integration test across platforms. If there are known acceptable failures, please add a PR comment linking to the appropriate issue.

## Update Release Notes 

Run the following script to update the release notes:

```shell
hack/release_notes.sh
```

Merge the output into CHANGELOG.md. See [PR#3175](https://github.com/kubernetes/minikube/pull/3175) as an example. Then get the PR submitted.

## Tag the Release

Run a command like this to tag it locally: `git tag -a v<version> -m "<version> Release"`.

And run a command like this to push the tag: `git push upstream v<version>`.

## Build the Release

This step uses the git tag to publish new binaries to GCS and create a github release:

 * navigate to the minikube "Release" jenkins job
 * Ensure that you are logged in (top right)
 * `VERSION_MAJOR`, `VERSION_MINOR`, and `VERSION_BUILD` should reflect the values in your Makefile
 * For `ISO_SHA256`, run: `gsutil cat gs://minikube/iso/minikube-v<version>.iso.sha256
 * Click *Build*

## Update releases.json

minikube-bot will send out a PR to update the release checksums at the top of `deploy/minikube/releases.json`. Make sure it is submitted, or hack one together yourself.

This file is used for auto-update notifications, but is not active until releases.json is copied to GCS.

## Upload releases.json to GCS

This step makes the new release trigger update notifications in old versions of Minikube.
Use this command from a clean git repo:

```shell
gsutil cp deploy/minikube/releases.json gs://minikube/releases.json
```

## Mark the release as `latest` in GCS:

```shell
gsutil cp -r 'gs://minikube/releases/$RELEASE/*' gs://minikube/releases/latest/
```

## Package managers which include minikube

These are downstream packages that are being maintained by others and how to upgrade them to make sure they have the latest versions

| Package Manager | URL | TODO |
| --- | --- | --- |
| Arch Linux AUR | https://aur.archlinux.org/packages/minikube/ | "Flag as package out-of-date"
| Brew Cask | https://github.com/Homebrew/homebrew-cask/blob/master/Casks/minikube.rb | Create a new PR in [Homebrew/homebrew-cask](https://github.com/Homebrew/homebrew-cask) with an updated version and SHA256

#### Updating the arch linux package
The Arch Linux AUR is maintained at https://aur.archlinux.org/packages/minikube/.  The installer PKGBUILD is hosted in its own repository.  The public read-only repository is hosted here `https://aur.archlinux.org/minikube.git` and the private read-write repository is hosted here `ssh://aur@aur.archlinux.org/minikube.git`

The repository is tracked in this repo under a submodule `installers/linux/arch_linux`.  Currently, its configured to point at the public readonly repository so if you want to push you should run this command to overwrite

`git config submodule.archlinux.url ssh://aur@aur.archlinux.org/minikube.git `

To actually update the package, you should bump the version and update the sha512 checksum.  You should also run `makepkg --printsrcinfo > .SRCINFO` to update the srcinfo file.  You can edit this manually if you don't have `makepkg` on your machine.

## Verification

After you've finished the release, run this command from the release commit to verify the release was done correctly:
`make check-release`.

## Update kubernetes.io docs

If there are major changes, please send a PR to update the official setup guide: [Running Kubernetes Locally via Minikube](https://kubernetes.io/docs/setup/minikube/)
