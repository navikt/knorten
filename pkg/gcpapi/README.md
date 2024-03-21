# Strategies for understanding the Google Cloud API
It can be difficult to understand the Google Cloud Platform API documentation, and the Go client libraries are not always easy to use as a result. Consider an example where we want to update the policy of a service account. The API documentation is [here](https://cloud.google.com/iam/docs/reference/rest/v1/projects.serviceAccounts/setIamPolicy), and the Go client library is [here](https://pkg.go.dev/google.golang.org/api/iam/v1?tab=doc#ProjectsServiceAccountsService.SetIamPolicy). However, there is little documentation of response codes, etc.

**Note:** not all the SDKs and API documentation are as challenging as this example, but the strategies can still be useful

## Using gcloud CLI
It can be useful to try running the equivalent `glcoud` by adding the `--log-http` flag to the command, e.g.:

```sh
$ gcloud iam service-accounts add-iam-policy-binding service --member meh --role meh --log-http
```

```sh
=======================
==== request start ====
uri: https://iam.googleapis.com/v1/projects/nada-dev-db2e/serviceAccounts/service:getIamPolicy?alt=json&options.requestedPolicyVersion=3
method: POST
== headers start ==
b'accept': b'application/json'
b'accept-encoding': b'gzip, deflate'
b'authorization': --- Token Redacted ---
b'content-length': b'0'
b'x-goog-api-client': b'cred-type/u'
== headers end ==
== body start ==

== body end ==
==== request end ====
---- response start ----
status: 404
-- headers start --
Alt-Svc: h3=":443"; ma=2592000,h3-29=":443"; ma=2592000
Cache-Control: private
Content-Encoding: gzip
Content-Type: application/json; charset=UTF-8
Date: Thu, 22 Feb 2024 08:31:10 GMT
Server: ESF
Transfer-Encoding: chunked
Vary: Origin, X-Origin, Referer
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
X-XSS-Protection: 0
-- headers end --
-- body start --
{
  "error": {
    "code": 404,
    "message": "Unknown service account",
    "status": "NOT_FOUND"
  }
}

-- body end --
total round trip time (request+response): 0.877 secs
---- response end ----
----------------------
ERROR: (gcloud.iam.service-accounts.add-iam-policy-binding) NOT_FOUND: Unknown service account
```

## Reading the terraform provider code
If you don't want to work out the details on your own, it can be useful to read the code of the terraform provider for Google Cloud as it is also written in go. For example, [here is the PR](https://github.com/hashicorp/terraform-provider-google/pull/171) that adds the `google_project_iam_binding` and `google_project_iam_member` resources.

- https://github.com/hashicorp/terraform-provider-google/tree/main
