# nvidia-runc-wrapper

A hyper-kludge to strip `NVIDIA_VISIBLE_DEVICES` environment variables from CRI `config.json` files, unless they are valid GPU UUIDs (where valid means UUIDs that validate).

This is meant to be used in concert with the [`nvidia-container-runtime`](https://github.com/NVIDIA/nvidia-container-runtime), and utilizes the same method to invoke the runtime as the NVIDIA project. That is, it uses `syscall.Exec` (under the hood `execve(2)`, per Go documentation) to replace itself with another process - namely `nvidia-container-runtime`.

## Objective
The purpose of this tool is as a workaround the way the official NVIDIA Kubernetes Device Plugin operates -- until it gets fixed.

At present, when Kubernetes sees a request with a resource of `nvidia.com/gpu` being >0, it calls out to the device plugin [via Kubelet, etc.] to allocate the resource. If the resource is 0 or not specified in the request, the allocation request is not sent. Hence, the only way for the NVIDIA Device Plugin to ever receive a request is if the resource `nvidia.com/gpu` is > 0.

The way GPUs are scheduled using the NVIDIA supported [NVIDIA Container Runtime](https://github.com/NVIDIA/nvidia-container-runtime), is by evaluating the container environment variable `NVIDIA_VISIBLE_DEVICES`. There are a number of valid options, including:

* `none`: do not allocate a GPU, but mount GPU tools
* `void`: do not allocate GPU, do not mount tools
* `all`: to mount all GPUs, and mount tools
* `0` or `0,1,2,...`: mount the device(s) by index, and mount tools
* `GPU-uuid` or `GPU-uuid,GPU-uuid,...`: mount the device(s) by UUID

The actual NVIDIA runtime is a modified version of `runc` that injects a hook, which in turn will check for the presence of the environment variable. If found, it'll launch another tool to actually allocate, mount, and place the GPU(s) into the container.

The problem arises when a container's `NVIDIA_VISIBLE_DEVICES` environment variable (e.g. as defined in a Dockerfile) is set to say, `all`, but on the Kubernetes side, there is no resource request for `nvidia.com/gpu`, then because of the way the NVIDIA Container Runtime and Device Plugins operate, a container will be assigned ALL the GPUs, thus circumventing any Kubernetes quotas, limits, etc. Most (if not all) of the official NVIDIA containers are built with the environment variable `NVIDIA_VISIBLE_DEVICES` set to `all`, and building from / extending these containers will thus all have the same environment variable set.

This tool, as mentioned, is a hyper-kludge that attempts to address the problem. It is intended as a drop-in replacement "container runtime", which will validate, and/or strip any environment variable named `NVIDIA_VISIBLE_DEVICES` if it doesn't explicitly specify GPUs by UUID. When it's done doing its work, it invokes the NVIDIA Container Runtime to continue as before.

## Installation
* Build this tool using the `scripts/build.sh` script
* Copy the binary to a GPU node
* Change the Docker `daemon.json`configuration file, and have the NVIDIA runtime invoke this binary

## Notes
Could this issue be addressed any other way (e.g. at the Kubernetes level)? Probably, yes. Two methods come to mind: 

* PodPresets
* A Mutating Admission Hook

The issue with PodPresets is that they are scoped at the Namespace level, which would require every namespace that uses GPUs to have a PodPreset installed, which makes it a bit of an operational headache. Assuming that wasn't an issue, then the PodPresent could've set the environment variable to `none`, but then again, a user can override it. It can try and set the resource of `nvidia.com/gpu` to 0, but the the allocation would never be called. Kinda back to square one.

A mutating web hook has similar problems, and edge cases need to be identified and worked with.

Ultimately the way NVIDIA has implemented this, it's kind of broken. Being that we're already using a sort-of broken system, the best place to duct-tape this seems to be where it's broken: at the Container Runtime level.

Google, on their NVIDIA Device Plugin does this correctly, and are not subject to this issue: they actually return a device mount response to an allocate request, and thus the mounting of GPU devices does not "leak".

### Links
* https://github.com/kubernetes/kubernetes/issues/59629
* https://github.com/kubernetes/kubernetes/issues/59631
* https://github.com/kubernetes/kubernetes/pull/59698
