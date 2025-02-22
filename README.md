# Storj V3 Network

[![Go Report Card](https://goreportcard.com/badge/storj.io/storj)](https://goreportcard.com/report/storj.io/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/storj.io/storj)
[![Coverage Status](https://img.shields.io/badge/coverage-master-green.svg)](https://build.dev.storj.io/job/storj/job/main/cobertura)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://github.com/storj/.github/assets/3217669/15b2f86d-e585-430f-83f8-67cccda07f73">
  <source media="(prefers-color-scheme: light)" srcset="https://github.com/storj/.github/assets/3217669/de7657b7-0497-4b72-8d71-99bf210164dc">
  <img alt="Storj logo" src="https://github.com/storj/.github/assets/3217669/de7657b7-0497-4b72-8d71-99bf210164dc" height="100">
</picture>

Storj is building a distributed cloud storage network.
[Check out our white paper for more info!](https://storj.io/storj.pdf)

----

Storj is an S3-compatible platform and suite of distributed applications that
allows you to store data in a secure and distributed manner. Your files are
encrypted, broken into little pieces and stored in a global distributed
network of computers. Luckily, we also support allowing you (and only you) to
retrieve those files!

## Table of Contents

- [Contributing](#contributing-to-storj)
- [Start using Storj](#start-using-storj)
- [License](#license)
- [Support](#support)

# Contributing to Storj

All of our code for Storj v3 is open source. If anything feels off, or if you feel that 
some functionality is missing, please check out the [contributing page](https://github.com/storj/storj/blob/main/CONTRIBUTING.md). 
There you will find instructions for sharing your feedback, building the tool locally, 
and submitting pull requests to the project.

### A Note about Versioning

While we are practicing [semantic versioning](https://semver.org/) for our client
libraries such as [uplink](https://github.com/storj/uplink), we are *not* practicing
semantic versioning in this repo, as we do not intend for it to be used via
[Go modules](https://blog.golang.org/using-go-modules). We may have
backwards-incompatible changes between minor and patch releases in this repo.

# Start using Storj

Our wiki has [documentation and tutorials](https://github.com/storj/storj/wiki).
Check out these three tutorials:

 * [Using the Storj Test Network](https://github.com/storj/storj/wiki/Test-network)
 * [Using the Uplink CLI](https://github.com/storj/storj/wiki/Uplink-CLI)
 * [Using the S3 Gateway](https://github.com/storj/storj/wiki/S3-Gateway)

# License

This repository is currently licensed with the [AGPLv3](https://www.gnu.org/licenses/agpl-3.0.en.html) license.

For code released under the AGPLv3, we request that contributors sign our
[Contributor License Agreement (CLA)](https://docs.google.com/forms/d/e/1FAIpQLSdVzD5W8rx-J_jLaPuG31nbOzS8yhNIIu4yHvzonji6NeZ4ig/viewform) so that we can relicense the
code under Apache v2, or other licenses in the future.

# Support

If you have any questions or suggestions please reach out to us on
[our community forum](https://forum.storj.io/) or file a ticket at
https://support.storj.io/.
