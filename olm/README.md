# Pre-Requisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [Operator SDK v0.10.0](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)

WARNING: Please make sure to use the precise `v0.10.0` version of the `operator-sdk`. If you use a more recent version, you might generate a CRD that is not compatible with Kubernetes 1.11 and Openshift 3.11 (see issue https://github.com/eclipse/che/issues/15396).

If these dependencies are not installed, `docker-run.sh` can be used as a container bootstrap to run a given script with the appropriate dependencies.

Example : `$ docker-run.sh update-nightly-olm-files.sh`


# Make new changes to OLM artifacts

Every change needs to be done in a new OLM artifact as previous artifacts are frozen.

A script is generating new folders/files that can be edited.

In `olm` folder

- If all dependencies are installed on the system:
``` 
$ update-nightly-olm-files.sh
```

- To use a docker environment
``` 
$ docker-run.sh update-nightly-olm-files.sh
```

Then the changes can be applied in the newly created CSV files.
