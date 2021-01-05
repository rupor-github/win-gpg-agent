----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
### This project needed some Windows specific functionality and original "https://github.com/foxcpp/go-assuan" was mostly Linux so I forked it
### at 5f169ecd9dc60f66706e60db814a48650ea53c3f and modified it slightly to see if this could be useful. I have no intention to submit PR 
### (since my modifications are not significant and specific), so I will keep it here with all original licensing intact.
----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

go-assuan
===========

[![Travis CI](https://img.shields.io/travis/com/foxcpp/go-assuan.svg?style=flat-square&logo=Linux)](https://travis-ci.com/foxcpp/go-assuan)
[![CodeCov](https://img.shields.io/codecov/c/github/foxcpp/go-assuan.svg?style=flat-square)](https://codecov.io/gh/foxcpp/go-assuan)
[![Issues](https://img.shields.io/github/issues-raw/foxcpp/go-assuan.svg?style=flat-square)](https://github.com/foxcpp/go-assuan/issues)
[![License](https://img.shields.io/github/license/foxcpp/go-assuan.svg?style=flat-square)](https://github.com/foxcpp/go-assuan/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/foxcpp/go-assuan)](https://goreportcard.com/report/github.com/foxcpp/go-assuan)

Pure Go implementation of Assuan IPC protocol.

Assuan protocol is used in GnuPG for communication between following
components: gpg, gpg-agent, pinentry, dirmngr. All of them are running as
separate processes and need a way to talk with each other. Assuan solves this
problem. 

Using this library you can talk to gpg-agent or dirmngr directly, invoke
pinentry to get password prompt similar to GnuPG's one and even use Assuan as a
protocol for your own IPC needs.

Assuan documentation: https://www.gnupg.org/documentation/manuals/assuan/index.html

Usage
-------

Main unit of communication in Assuan is a command. Command consists of comamnd
name (typically in uppercase) and one or more parameters (represented as one
string). Each command sent by client have response with status ("OK" or error)
and optional arbitrary data. 

In additional to simple commands there are transactions (data inquiries).
This is one way to send large data streams from client. Transaction is
initiated by command and then server will request data using keywords (actually
one keyword per stream). Client can cancel data transmission at any time. At
the end of transaction server will send result (OK or error). go-assuan supports
optional data stream as a result but this is not in use by any of protocol
implementers.

Protocol also specifies file descriptor passing but this is not supported by
library yet.

Client side example:
```go
// Connect to dirmngr.
conn, _ := net.Dial("unix", ".gnupg/S.dirmngr")
ses, _ := assuan.Init(conn)
defer ses.Close()

// Search for my key on default keyserver.
data, _ := ses.SimpleCmd("KS_SEARCH", "foxcpp")
fmt.Println(string(data))
// data []byte = "info:1:1%0Apub:2499BEB8B47B0235009A5F0AEE8384B0561A25AF:..."

// More complex transaction: send key to keyserver.
ses.Transact("KS_PUT", "", map[string][]byte{
	"KEYBLOCK":      []byte{},
	"KEYBLOCK_INFO": []byte{},
})
```

Server code is much more complex, see it [here](server/server_test.go).

Versioning & Git
---------

go-assuan follows [Semantic Versioning 2.0.0](https://semver.org). `master` branch contains
latest **pre-release**. `dev` branch contains bleeding edge code for next release. For stable
code use Git tags.

License
---------

MIT.
