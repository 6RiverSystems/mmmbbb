# Go Six Fault Injection

The `go.6river.tech/gosix/faults` package has a very basic fault injection
framework. The gRPC support code includes middleware to allow injecting faults
for any gRPC call.

A fault injection is composed of an "operation" (a `string`), and an optional
"parameter set" (a `map[string]string`). To match an actual call, the operation
must match exactly, and any parameters in the injection must match the actual
parameters from the call. The call may have extra parameters.

A fault injection finally has a `count` — the number of times the fault should
be injected, after which the injection will be removed from the configuration.

## gRPC Middleware

The default app setup inserts gRPC interceptors to allow fault injection for
unary and server streaming calls for gRPC servers hosted by the app (not for
gRPC clients the app uses!).

The operation for a gRPC call is the unqualified method name. The parameter set
will have:

- `service` ⇒ `method`
  - This allows filtering on specific service names when matching the method, in
    case multiple services have the same method name in the application.
- For each field in the request protobuf object with `StringKind` value:
  - `TextName` ⇒ value
  - `JSONName` ⇒ value
  - `Name` ⇒ value
  - `FullName` ⇒ value
