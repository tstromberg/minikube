/*
Copyright 2020 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the Reason{ID: "License", ExitCode: },);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an Reason{ID: "AS IS", ExitCode: }, BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reason

type Reason struct {
	ID string
	ExitCode int
}


var (
	DrvCpEndpoint  = Reason{ID: "DRV_CP_ENDPOINT", ExitCode: ExDriverError},
	DrvPortForward = Reason{ID: "DRV_PORT_FORWARD", ExitCode: ExDriverError},

	GuestCacheLoad       = Reason{ID: "GUEST_CACHE_LOAD", ExitCode: ExGuestError },
	GuestCert            = Reason{ID: "GUEST_CERT", ExitCode: ExGuestError },
	GuestCpConfig        = Reason{ID: "GUEST_CP_CONFIG", ExitCode: ExGuestConfig },
	GuestDeletion        = Reason{ID: "GUEST_DELETION", ExitCode: ExGuestError },
	GuestLoadHost        = Reason{ID: "GUEST_LOAD_HOST", ExitCode: ExGuestError },
	GuestMount           = Reason{ID: "GUEST_MOUNT", ExitCode: ExGuestError },
	GuestNodeAdd         = Reason{ID: "GUEST_NODE_ADD", ExitCode: ExGuestError },
	GuestNodeDelete      = Reason{ID: "GUEST_NODE_DELETE", ExitCode: ExGuestError },
	GuestNodeProvision   = Reason{ID: "GUEST_NODE_PROVISION", ExitCode: ExGuestError },
	GuestNodeRetrieve    = Reason{ID: "GUEST_NODE_RETRIEVE", ExitCode: ExGuestError },
	GuestNodeStart       = Reason{ID: "GUEST_NODE_START", ExitCode: ExGuestError },
	GuestPause           = Reason{ID: "GUEST_PAUSE", ExitCode: ExGuestError },
	GuestProfileDeletion = Reason{ID: "GUEST_PROFILE_DELETION", ExitCode: ExGuestError },
	GuestProvision       = Reason{ID: "GUEST_PROVISION", ExitCode: ExGuestError },
	GuestStart           = Reason{ID: "GUEST_START", ExitCode: ExGuestError },
	GuestStatus          = Reason{ID: "GUEST_STATUS", ExitCode: ExGuestError },
	GuestStopTimeout     = Reason{ID: "GUEST_STOP_TIMEOUT", ExitCode: ExGuestTimeout },
	GuestUnpause         = Reason{ID: "GUEST_UNPAUSE", ExitCode: ExGuestError },

	HostBrowser          = Reason{ID: "HOST_BROWSER", ExitCode: ExHostError },
	HostConfigLoad       = Reason{ID: "HOST_CONFIG_LOAD", ExitCode: ExHostConfig },
	HostCurrentUser      = Reason{ID: "HOST_CURRENT_USER", ExitCode: ExHostConfig },
	HostDelCache         = Reason{ID: "HOST_DEL_CACHE", ExitCode: ExHostError },
	HostKillMountProc    = Reason{ID: "HOST_KILL_MOUNT_PROC", ExitCode: ExHostError },
	HostKubeconfigUnset  = Reason{ID: "HOST_KUBECNOFIG_UNSET", ExitCode: ExHostConfig },
	HostKubeconfigUpdate = Reason{ID: "HOST_KUBECONFIG_UPDATE", ExitCode: ExHostConfig },
	HostKubectlProxy     = Reason{ID: "HOST_KUBECTL_PROXY", ExitCode: ExHostError },
	HostMkdirHome        = Reason{ID: "HOST_MKDIR_HOME", ExitCode: ExHostPermission },
	HostMountPid         = Reason{ID: "HOST_MOUNT_PID", ExitCode: ExHostError},
	HostPathMissing      = Reason{ID: "HOST_PATH_MISSING", ExitCode: ExHostNotFound},
	HostPathStat         = Reason{ID: "HOST_PATH_STAT", ExitCode: ExHostError},
	HostPurge            = Reason{ID: "HOST_PURGE", ExitCode: ExHostError},
	HostSaveProfile      = Reason{ID: "HOST_SAVE_PROFILE", ExitCode: ExHostConfig},

	IfHostIp    = Reason{ID: "IF_HOST_IP", ExitCode: ExLocalNetworkError},
	IfMountIp   = Reason{ID: "IF_MOUNT_IP", ExitCode:  ExLocalNetworkError},
	IfMountPort = Reason{ID: "IF_MOUNT_PORT", ExitCode:  ExLocalNetworkError},
	IfSSHClient = Reason{ID: "IF_SSH_CLIENT", ExitCode:  ExLocalNetworkError},


	InetCacheBinaries    = Reason{ID: "INET_CACHE_BINARIES", ExitCode: ExInternetError},
	InetCacheKubectl     = Reason{ID: "INET_CACHE_KUBECTL", ExitCode: ExInternetError},
	InetCacheTar         = Reason{ID: "INET_CACHE_TAR", ExitCode: ExInternetError},
	InetGetVersions      = Reason{ID: "INET_GET_VERSIONS", ExitCode: ExInternetError},
	InetRepo             = Reason{ID: "INET_REPO", ExitCode: ExInternetError},
	InetReposUnavailable = Reason{ID: "INET_REPOS_UNAVAILABLE", ExitCode: ExInternetError},



	MkAddonEnable     = Reason{ID: "MK_ADDON_ENABLE", ExitCode: },
	MkAddConfig       = Reason{ID: "MK_ADD_CONFIG", ExitCode: },
	MkBindFlags       = Reason{ID: "MK_BIND_FLAGS", ExitCode: },
	MkBootstrapper    = Reason{ID: "MK_BOOTSTRAPPER", ExitCode: },
	MkCacheList       = Reason{ID: "MK_CACHE_LIST", ExitCode: },
	MkCacheLoad       = Reason{ID: "MK_CACHE_LOAD", ExitCode: },
	MkCmdRunner       = Reason{ID: "MK_CMD_RUNNER", ExitCode: },
	MkCommandRunner   = Reason{ID: "MK_COMMAND_RUNNER", ExitCode: },
	MkConfigUnset     = Reason{ID: "MK_CONFIG_UNSET", ExitCode: },
	MkConfigView      = Reason{ID: "MK_CONFIG_VIEW", ExitCode: },
	MkDelConfig       = Reason{ID: "MK_DEL_CONFIG", ExitCode: },
	MkDisable         = Reason{ID: "MK_DISABLE", ExitCode: },
	MkDockerScript    = Reason{ID: "MK_DOCKER_SCRIPT", ExitCode: },
	MkEnable          = Reason{ID: "MK_ENABLE", ExitCode: },
	MkFlagsBind       = Reason{ID: "MK_FLAGS_BIND", ExitCode: },
	MkFlagsSet        = Reason{ID: "MK_FLAGS_SET", ExitCode: },
	MkFormatUsage     = Reason{ID: "MK_FORMAT_USAGE", ExitCode: },
	MkGenerateDocs    = Reason{ID: "MK_GENERATE_DOCS", ExitCode: },
	MkJsonMarshal     = Reason{ID: "MK_JSON_MARSHAL", ExitCode: },
	MkListConfig      = Reason{ID: "MK_LIST_CONFIG", ExitCode: },
	MkLogtostderrFlag = Reason{ID: "MK_LOGTOSTDERR_FLAG", ExitCode: },
	MkLogFollow       = Reason{ID: "MK_LOG_FOLLOW", ExitCode: },
	MkMachineApi      = Reason{ID: "MK_MACHINE_API", ExitCode: },
	MkNewRuntime      = Reason{ID: "MK_NEW_RUNTIME", ExitCode: },
	MkOutputUsage     = Reason{ID: "MK_OUTPUT_USAGE", ExitCode: },
	MkRuntime         = Reason{ID: "MK_RUNTIME", ExitCode: },
	MkSet             = Reason{ID: "MK_SET", ExitCode: },
	MkSetScript       = Reason{ID: "MK_SET_SCRIPT", ExitCode: },
	MkShellDetect     = Reason{ID: "MK_SHELL_DETECT", ExitCode: },
	MkStatusJson      = Reason{ID: "MK_STATUS_JSON", ExitCode: },
	MkStatusText      = Reason{ID: "MK_STATUS_TEXT", ExitCode: },
	MkUnsetScript     = Reason{ID: "MK_UNSET_SCRIPT", ExitCode: },
	MkViewExec        = Reason{ID: "MK_VIEW_EXEC", ExitCode: },
	MkViewTmpl        = Reason{ID: "MK_VIEW_TMPL", ExitCode: },
	MkYamlMarshal     = Reason{ID: "MK_YAML_MARSHAL", ExitCode: },

	RuntimeEnable     = Reason{ID: "RUNTIME_ENABLE", ExitCode: ExRuntimeError},
	RuntimeCache = Reason{ID: "RUNTIME_CACHE", ExitCode: ExRuntimeError},

	SvcCheckTimeout = Reason{ID: "SVC_CHECK_TIMEOUT", ExitCode: ExSvcTimeout},
	SvcTimeout      = Reason{ID: "SVC_TIMEOUT", ExitCode: ExSvcTimeout},
	SvcTunnelStart  = Reason{ID: "SVC_TUNNEL_START", ExitCode: ExSvcError},
	SvcTunnelStop   = Reason{ID: "SVC_TUNNEL_STOP", ExitCode: ExSvcError},
	SvcUrlTimeout   = Reason{ID: "SVC_URL_TIMEOUT", ExitCode: ExSvcTimeout},
)
