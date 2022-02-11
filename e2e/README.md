### Shipwright imgctrl e2e tests

This directory contains a series of end to end tests designed to test imgctrl basic features.
These tests leverage [kuttl](https://kuttl.dev/) tool. In order to run these tests you have to
have imgctrl already deployed in a cluster that you have access to.

Some tooling is in place to assure the right version for `kuttl` is installed, to install kuttl
run:

```
$ make get-kuttl
```

This will install `kuttl` under `output` directory. With `kuttl` installed you can then run:

```
$ make e2e
```

For further information on `kuttl` please refer to its [doc](https://kuttl.dev/docs/). These
tests will change the deployed Shipwright Image config so don't expect that at the end of the
e2d you still have the same configuration. During github's CI we deploy a Kind instance and
run these on top of that (see .github directory for further info).
