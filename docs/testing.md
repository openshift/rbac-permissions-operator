# Testing

Tests are playing a primary role and we take them seriously.
It is expected from PRs to add, modify or delete tests on case by case scenario.
To contribute you need to be familiar with:

* [Ginkgo](https://github.com/onsi/ginkgo) - BDD Testing Framework for Go
* [Gomega](https://onsi.github.io/gomega/) - Matcher/assertion library

## Prerequisites

The prerequisites for testing i.e, `ginkgo` and `mockgen` binaries can be setup by running `make tools` command locally.

## Bootstrapping the tests

```bash
$ cd pkg/dummy
$ ginkgo bootstrap
$ ginkgo generate dummy.go

find .
./dummy.go
./dummy_suite_test.go
./dummy_test.go
```

## How to run the tests

* You can run the tests using `./boilerplate/_lib/container-make test` or `go test ./...`
* Can also make use of the `ginkgo` command to run tests for a package. Example:

```bash
ginkgo -v pkg/controller/subjectpermission
ginkgo -v pkg/controller/namespace
```

## Writing tests

### Mocking interfaces

This project makes use of [`GoMock`](https://github.com/golang/mock) to mock service interfaces. This comes with the `mockgen` utility which can be used to generate or re-generate mock interfaces that can be used to simulate the behaviour of an external dependency.

Once installed, an interface can be mocked by running:

```bash
mockgen -s=/path/to/file_containing_interface.go > /path/to/output_mock_file.go
```

However, it is considered good practice to include a [go generate](https://golang.org/pkg/cmd/go/internal/generate/) directive above the interface which defines the specific `mockgen` command that will generate your mocked interface.

Internal interfaces are mocked using this method. When making changes to these packages, you should re-generate the mocks to ensure they too are updated. This can be performed manually by running `go generate /path/to/file.go` or for the whole project via `make generate`.
